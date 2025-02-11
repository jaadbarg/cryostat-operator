// Copyright The Cryostat Authors
//
// The Universal Permissive License (UPL), Version 1.0
//
// Subject to the condition set forth below, permission is hereby granted to any
// person obtaining a copy of this software, associated documentation and/or data
// (collectively the "Software"), free of charge and under any and all copyright
// rights in the Software, and any and all patent rights owned or freely
// licensable by each licensor hereunder covering either (i) the unmodified
// Software as contributed to or provided by such licensor, or (ii) the Larger
// Works (as defined below), to deal in both
//
// (a) the Software, and
// (b) any piece of software and/or hardware listed in the lrgrwrks.txt file if
// one is included with the Software (each a "Larger Work" to which the Software
// is contributed by such licensors),
//
// without restriction, including without limitation the rights to copy, create
// derivative works of, display, perform, and distribute the Software and make,
// use, sell, offer for sale, import, export, have made, and have sold the
// Software and the Larger Work(s), and to sublicense the foregoing rights on
// either these or other terms.
//
// This license is subject to the following condition:
// The above copyright notice and either this complete permission notice or at
// a minimum a reference to the UPL must be included in all copies or
// substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package controllers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/cryostatio/cryostat-operator/api/v1beta1"

	goerrors "errors"

	"github.com/cryostatio/cryostat-operator/internal/controllers/common"
	resources "github.com/cryostatio/cryostat-operator/internal/controllers/common/resource_definitions"
	consolev1 "github.com/openshift/api/console/v1"
	openshiftv1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// CryostatReconciler reconciles a Cryostat object
type CryostatReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	IsOpenShift   bool
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
	common.ReconcilerTLS
}

// Name used for Finalizer that handles Cryostat deletion
const cryostatFinalizer = "operator.cryostat.io/cryostat.finalizer"

// Environment variable to override the core application image
const coreImageTagEnv = "RELATED_IMAGE_CORE"

// Environment variable to override the JFR datasource image
const datasourceImageTagEnv = "RELATED_IMAGE_DATASOURCE"

// Environment variable to override the Grafana dashboard image
const grafanaImageTagEnv = "RELATED_IMAGE_GRAFANA"

// Regular expression for the start of a GID range in the OpenShift
// supplemental groups SCC annotation
var supGroupRegexp = regexp.MustCompile(`^\d+`)

// +kubebuilder:rbac:namespace=system,groups="",resources=pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts,verbs=*
// +kubebuilder:rbac:namespace=system,groups="",resources=replicationcontrollers,verbs=get
// +kubebuilder:rbac:namespace=system,groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=create;get;list;update;watch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=create;get;list;update;watch;delete
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=selfsubjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:namespace=system,groups=route.openshift.io,resources=routes;routes/custom-host,verbs=*
// +kubebuilder:rbac:namespace=system,groups=apps.openshift.io,resources=deploymentconfigs,verbs=get
// +kubebuilder:rbac:namespace=system,groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=*
// +kubebuilder:rbac:namespace=system,groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:namespace=system,groups=cert-manager.io,resources=issuers;certificates,verbs=create;get;list;update;watch
// +kubebuilder:rbac:namespace=system,groups=operator.cryostat.io,resources=cryostats,verbs=*
// +kubebuilder:rbac:namespace=system,groups=operator.cryostat.io,resources=cryostats/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=system,groups=operator.cryostat.io,resources=cryostats/finalizers,verbs=update
// +kubebuilder:rbac:groups=console.openshift.io,resources=consolelinks,verbs=get;create;list;update;delete
// +kubebuilder:rbac:namespace=system,groups=networking.k8s.io,resources=ingresses,verbs=*

// Reconcile processes a Cryostat CR and manages a Cryostat installation accordingly
func (r *CryostatReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	reqLogger.Info("Reconciling Cryostat")

	// Fetch the Cryostat instance
	instance := &operatorv1beta1.Cryostat{}
	err := r.Client.Get(context.Background(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Cryostat instance not found")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Error reading Cryostat instance")
		return reconcile.Result{}, err
	}

	// Check if this Cryostat is being deleted
	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, cryostatFinalizer) {
			err = r.deleteClusterRoleBinding(ctx, instance)
			if err != nil {
				return reconcile.Result{}, err
			}

			// OpenShift-specific
			if r.IsOpenShift {
				err = r.deleteConsoleLink(ctx, instance)
				if err != nil {
					return reconcile.Result{}, err
				}
			}

			err = common.RemoveFinalizer(ctx, r.Client, instance, cryostatFinalizer)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		// Ready for deletion
		return reconcile.Result{}, nil
	}

	// Add our finalizer, so we can clean up Cryostat resources upon deletion
	if !controllerutil.ContainsFinalizer(instance, cryostatFinalizer) {
		err := common.AddFinalizer(context.Background(), r.Client, instance, cryostatFinalizer)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("Spec", "Minimal", instance.Spec.Minimal)

	pvc := resources.NewPersistentVolumeClaimForCR(instance)
	if err := controllerutil.SetControllerReference(instance, pvc, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
		return reconcile.Result{}, err
	}

	grafanaSecret := resources.NewGrafanaSecretForCR(instance)
	if err := controllerutil.SetControllerReference(instance, grafanaSecret, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: grafanaSecret.Name, Namespace: grafanaSecret.Namespace}, &corev1.Secret{}, grafanaSecret); err != nil {
		return reconcile.Result{}, err
	}

	jmxAuthSecret := resources.NewJmxSecretForCR(instance)
	if err := controllerutil.SetControllerReference(instance, jmxAuthSecret, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: jmxAuthSecret.Name, Namespace: jmxAuthSecret.Namespace}, &corev1.Secret{}, jmxAuthSecret); err != nil {
		return reconcile.Result{}, err
	}

	// Set up TLS using cert-manager, if available
	var tlsConfig *resources.TLSConfig
	var routeTLS *openshiftv1.TLSConfig
	if r.IsCertManagerEnabled(instance) {
		tlsConfig, err = r.setupTLS(context.Background(), instance)
		if err != nil {
			if err == common.ErrCertNotReady {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}
			reqLogger.Error(err, "Failed to set up TLS for Cryostat")
			return reconcile.Result{}, err
		}

		// Get CA certificate from secret and set as destination CA in route
		caCert, err := r.GetCryostatCABytes(context.Background(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		routeTLS = &openshiftv1.TLSConfig{
			Termination:              openshiftv1.TLSTerminationReencrypt,
			DestinationCACertificate: string(caCert),
		}
	}

	// Create RBAC resources for Cryostat
	err = r.createRBAC(ctx, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	serviceSpecs := &resources.ServiceSpecs{}
	if !instance.Spec.Minimal {
		grafanaSvc := resources.NewGrafanaService(instance)
		url, err := r.createService(context.Background(), instance, grafanaSvc, &grafanaSvc.Spec.Ports[0], routeTLS)
		if err != nil {
			return requeueIfIngressNotReady(reqLogger, err)
		}
		serviceSpecs.GrafanaURL = url

		// check for existing minimal deployment and delete if found
		deployment := &appsv1.Deployment{}
		err = r.Client.Get(context.Background(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
		if err == nil && len(deployment.Spec.Template.Spec.Containers) == 1 {
			reqLogger.Info("Deleting existing minimal deployment")
			err = r.Client.Delete(context.Background(), deployment)
			if err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
			}
		}
	} else {
		// check for existing non-minimal resources and delete if found
		svc := resources.NewGrafanaService(instance)
		if r.IsOpenShift {
			reqLogger.Info("Deleting existing non-minimal route", "route.Name", svc.Name)
			route := &openshiftv1.Route{}
			err = r.Client.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, route)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Info("Non-minimal route could not be retrieved", "route.Name", svc.Name)
				return reconcile.Result{}, err
			} else if err == nil {
				err = r.Client.Delete(context.Background(), route)
				if err != nil && !errors.IsNotFound(err) {
					reqLogger.Info("Could not delete non-minimal route", "route.Name", svc.Name)
					return reconcile.Result{}, err
				}
			}
		} else {
			reqLogger.Info("Deleting existing non-minimal ingress", "ingress.Name", svc.Name)
			ingress := &netv1.Ingress{}
			err = r.Client.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, ingress)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Info("Non-minimal ingress could not be retrieved", "ingress.Name", svc.Name)
				return reconcile.Result{}, err
			} else if err == nil {
				err = r.Client.Delete(context.Background(), ingress)
				if err != nil && !errors.IsNotFound(err) {
					reqLogger.Info("Could not delete non-minimal ingress", "ingress.Name", svc.Name)
					return reconcile.Result{}, err
				}
			}
		}

		err = r.Client.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, svc)
		if err == nil {
			reqLogger.Info("Deleting existing non-minimal service", "svc.Name", svc.Name)
			err = r.Client.Delete(context.Background(), svc)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Info("Could not delete non-minimal service")
				return reconcile.Result{}, err
			}
		}

		deployment := &appsv1.Deployment{}
		err = r.Client.Get(context.Background(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
		if err == nil && len(deployment.Spec.Template.Spec.Containers) > 1 {
			reqLogger.Info("Deleting existing non-minimal deployment")
			err = r.Client.Delete(context.Background(), deployment)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Info("Could not delete non-minimal deployment")
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
			}
		}
	}

	exporterSvc := resources.NewExporterService(instance)
	url, err := r.createService(context.Background(), instance, exporterSvc, &exporterSvc.Spec.Ports[0], routeTLS)
	if err != nil {
		return requeueIfIngressNotReady(reqLogger, err)
	}
	serviceSpecs.CoreURL = url

	cmdChanSvc := resources.NewCommandChannelService(instance)
	url, err = r.createService(context.Background(), instance, cmdChanSvc, &cmdChanSvc.Spec.Ports[0], routeTLS)
	if err != nil {
		return requeueIfIngressNotReady(reqLogger, err)
	}
	serviceSpecs.CommandURL = url

	imageTags := r.getImageTags()
	fsGroup, err := r.getFSGroup(ctx, instance.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	deployment := resources.NewDeploymentForCR(instance, serviceSpecs, imageTags, tlsConfig, *fsGroup)
	podTemplate := deployment.Spec.Template.DeepCopy()
	if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Update pod template spec to propagate any changes from Cryostat CR
		deployment.Spec.Template.Spec = podTemplate.Spec
		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	reqLogger.Info(fmt.Sprintf("Deployment %s", op))

	if serviceSpecs.CoreURL != nil {
		instance.Status.ApplicationURL = serviceSpecs.CoreURL.String()
		err = r.Client.Status().Update(context.Background(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// OpenShift-specific
	if r.IsOpenShift {
		err := r.createConsoleLink(ctx, instance, serviceSpecs.CoreURL.String())
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("Successfully reconciled deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CryostatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1beta1.Cryostat{})

	// Watch for changes to secondary resources and requeue the owner Cryostat
	resources := []client.Object{&appsv1.Deployment{}, &corev1.Service{}, &corev1.Secret{}, &corev1.PersistentVolumeClaim{}}
	if r.IsOpenShift {
		resources = append(resources, &openshiftv1.Route{})
	}
	// TODO watch certificates and redeploy when renewed

	for _, resource := range resources {
		c = c.Watches(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &operatorv1beta1.Cryostat{},
		})
	}

	return c.Complete(r)
}

func (r *CryostatReconciler) createService(ctx context.Context, controller *operatorv1beta1.Cryostat, svc *corev1.Service, exposePort *corev1.ServicePort,
	tlsConfig *openshiftv1.TLSConfig) (*url.URL, error) {
	if err := controllerutil.SetControllerReference(controller, svc, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.createObjectIfNotExists(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &corev1.Service{}, svc); err != nil {
		return nil, err
	}

	// Use edge termination by default
	if tlsConfig == nil {
		tlsConfig = &openshiftv1.TLSConfig{
			Termination:                   openshiftv1.TLSTerminationEdge,
			InsecureEdgeTerminationPolicy: openshiftv1.InsecureEdgeTerminationPolicyRedirect,
		}
	}
	if r.IsOpenShift {
		return r.createRouteForService(ctx, controller, svc, *exposePort, tlsConfig)
	} else {
		if controller.Spec.NetworkOptions == nil {
			return nil, nil
		}
		networkConfig, err := getNetworkConfig(controller, svc)
		if err != nil {
			return nil, err
		}
		if networkConfig == nil || networkConfig.IngressSpec == nil {
			return nil, nil
		}
		return r.createIngressForService(controller, svc, networkConfig)
	}
}

// ErrIngressNotReady is returned when Kubernetes has not yet exposed our services
// so that they may be accessed outside of the cluster
var ErrIngressNotReady = goerrors.New("Ingress configuration not yet available")

func (r *CryostatReconciler) createRouteForService(ctx context.Context, cr *operatorv1beta1.Cryostat,
	svc *corev1.Service, exposePort corev1.ServicePort, tlsConfig *openshiftv1.TLSConfig) (*url.URL, error) {
	logger := r.Log.WithValues("Request.Namespace", svc.Namespace, "Name", svc.Name, "Kind", fmt.Sprintf("%T", &openshiftv1.Route{}))
	route := &openshiftv1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
	}
	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return nil, err
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		// Update Route spec
		route.Spec = openshiftv1.RouteSpec{
			To: openshiftv1.RouteTargetReference{
				Kind: "Service",
				Name: svc.Name,
			},
			Port: &openshiftv1.RoutePort{TargetPort: exposePort.TargetPort},
			TLS:  tlsConfig,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("Route %s", op), "Service.Status", fmt.Sprintf("%#v", route.Status))
	if len(route.Status.Ingress) < 1 {
		return nil, ErrIngressNotReady
	}

	return &url.URL{
		Scheme: getProtocol(tlsConfig),
		Host:   route.Status.Ingress[0].Host,
	}, nil
}

func (r *CryostatReconciler) createIngressForService(controller *operatorv1beta1.Cryostat, svc *corev1.Service,
	networkConfig *operatorv1beta1.NetworkConfiguration) (*url.URL, error) {
	logger := r.Log.WithValues("Request.Namespace", svc.Namespace, "Name", svc.Name, "Kind", fmt.Sprintf("%T", &netv1.Ingress{}))

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Annotations: networkConfig.Annotations,
			Labels:      networkConfig.Labels,
		},
		Spec: *networkConfig.IngressSpec,
	}
	if err := controllerutil.SetControllerReference(controller, ingress, r.Scheme); err != nil {
		return nil, err
	}

	found := &netv1.Ingress{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Not found")
		if err := r.Client.Create(context.Background(), ingress); err != nil {
			logger.Error(err, "Could not be created")
			return nil, err
		}
		logger.Info("Created")
		found = ingress
	} else if err != nil {
		logger.Error(err, "Could not be read")
		return nil, err
	}

	logger.Info("Ingress created", "Service.Status", fmt.Sprintf("%#v", found.Status))
	host := ""
	if networkConfig.IngressSpec.Rules != nil && networkConfig.IngressSpec.Rules[0].Host != "" {
		host = networkConfig.IngressSpec.Rules[0].Host
	}

	scheme := "http"
	if networkConfig.IngressSpec.TLS != nil {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   host,
	}, nil
}

func (r *CryostatReconciler) createObjectIfNotExists(ctx context.Context, ns types.NamespacedName, found client.Object, toCreate client.Object) error {
	logger := r.Log.WithValues("Request.Namespace", ns.Namespace, "Name", ns.Name, "Kind", fmt.Sprintf("%T", toCreate))
	err := r.Client.Get(ctx, ns, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Not found")
		if err := r.Client.Create(ctx, toCreate); err != nil {
			logger.Error(err, "Could not be created")
			return err
		} else {
			logger.Info("Created")
			found = toCreate
		}
	} else if err != nil {
		logger.Error(err, "Could not be read")
		return err
	}
	logger.Info("Already exists")
	return nil
}

func (r *CryostatReconciler) createRBAC(ctx context.Context, cr *operatorv1beta1.Cryostat) error {
	// Create ServiceAccount
	sa := resources.NewServiceAccountForCR(cr)
	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return err
	}
	if err := r.createObjectIfNotExists(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace},
		&corev1.ServiceAccount{}, sa); err != nil {
		return err
	}

	// Create Role
	role := resources.NewRoleForCR(cr)
	if err := controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return err
	}
	if err := r.createObjectIfNotExists(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace},
		&rbacv1.Role{}, role); err != nil {
		return err
	}

	// Create RoleBinding
	binding := resources.NewRoleBindingForCR(cr)
	if err := controllerutil.SetControllerReference(cr, binding, r.Scheme); err != nil {
		return err
	}
	if err := r.createObjectIfNotExists(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
		&rbacv1.RoleBinding{}, binding); err != nil {
		return err
	}

	// Create ClusterRoleBinding
	clusterBinding := resources.NewClusterRoleBindingForCR(cr)
	if err := r.createObjectIfNotExists(ctx, types.NamespacedName{Name: clusterBinding.Name},
		&rbacv1.ClusterRoleBinding{}, clusterBinding); err != nil {
		return err
	}
	// ClusterRoleBinding can't be owned by namespaced CR, clean up using finalizer

	return nil
}

func (r *CryostatReconciler) deleteClusterRoleBinding(ctx context.Context, cr *operatorv1beta1.Cryostat) error {
	reqLogger := r.Log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.Name)

	clusterBinding := resources.NewClusterRoleBindingForCR(cr)
	err := r.Delete(ctx, clusterBinding)
	if err != nil {
		reqLogger.Error(err, "failed to delete ClusterRoleBinding", "bindingName", clusterBinding.Name)
		return err
	}
	reqLogger.Info("deleted ClusterRoleBinding", "bindingName", clusterBinding.Name)
	return nil
}

func (r *CryostatReconciler) getImageTags() *resources.ImageTags {
	return &resources.ImageTags{
		CoreImageTag:       r.getEnvOrDefault(coreImageTagEnv, resources.DefaultCoreImageTag),
		DatasourceImageTag: r.getEnvOrDefault(datasourceImageTagEnv, resources.DefaultDatasourceImageTag),
		GrafanaImageTag:    r.getEnvOrDefault(grafanaImageTagEnv, resources.DefaultGrafanaImageTag),
	}
}

func (r *CryostatReconciler) getEnvOrDefault(name string, defaultVal string) string {
	val := r.GetEnv(name)
	if len(val) > 0 {
		return val
	}
	return defaultVal
}

// fsGroup to use when not constrained
const defaultFSGroup int64 = 18500

func (r *CryostatReconciler) getFSGroup(ctx context.Context, namespace string) (*int64, error) {
	if r.IsOpenShift {
		// Check namespace for supplemental groups annotation
		ns := &corev1.Namespace{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: namespace}, ns)
		if err != nil {
			return nil, err
		}

		supGroups, found := ns.Annotations[securityv1.SupplementalGroupsAnnotation]
		if found {
			return parseSupGroups(supGroups)
		}
	}
	fsGroup := defaultFSGroup
	return &fsGroup, nil
}

func parseSupGroups(supGroups string) (*int64, error) {
	// Extract the start value from the annotation
	match := supGroupRegexp.FindString(supGroups)
	if len(match) == 0 {
		return nil, fmt.Errorf("no group ID found in %s annotation",
			securityv1.SupplementalGroupsAnnotation)
	}
	gid, err := strconv.ParseInt(match, 10, 64)
	if err != nil {
		return nil, err
	}
	return &gid, nil
}

func (r *CryostatReconciler) createConsoleLink(ctx context.Context, cr *operatorv1beta1.Cryostat, url string) error {
	link := resources.NewConsoleLink(cr, url)
	return r.createObjectIfNotExists(ctx, types.NamespacedName{Name: link.Name}, &consolev1.ConsoleLink{}, link)
}

func (r *CryostatReconciler) deleteConsoleLink(ctx context.Context, cr *operatorv1beta1.Cryostat) error {
	reqLogger := r.Log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.Name)
	link := resources.NewConsoleLink(cr, "")
	err := r.Client.Delete(ctx, link)
	if err != nil {
		reqLogger.Error(err, "failed to delete ConsoleLink", "linkName", link.Name)
		return err
	}
	reqLogger.Info("deleted ConsoleLink", "linkName", link.Name)
	return nil
}

func getProtocol(tlsConfig *openshiftv1.TLSConfig) string {
	if tlsConfig == nil {
		return "http"
	}
	return "https"
}

func requeueIfIngressNotReady(log logr.Logger, err error) (reconcile.Result, error) {
	if err == ErrIngressNotReady {
		log.Info(err.Error())
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return reconcile.Result{}, err
}

func getNetworkConfig(controller *operatorv1beta1.Cryostat, svc *corev1.Service) (*operatorv1beta1.NetworkConfiguration, error) {
	if svc.Name == controller.Name {
		return controller.Spec.NetworkOptions.CoreConfig, nil
	} else if svc.Name == controller.Name+"-command" {
		return controller.Spec.NetworkOptions.CommandConfig, nil
	} else if svc.Name == controller.Name+"-grafana" {
		return controller.Spec.NetworkOptions.GrafanaConfig, nil
	} else {
		return nil, goerrors.New("Service name not recognized")
	}
}
