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

package test

import (
	"time"

	operatorv1beta1 "github.com/cryostatio/cryostat-operator/api/v1beta1"
	"github.com/cryostatio/cryostat-operator/internal/controllers/common/resource_definitions"
	certv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	"github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

func NewTestScheme() *runtime.Scheme {
	s := scheme.Scheme

	// Add all APIs used by the operator to the scheme
	sb := runtime.NewSchemeBuilder(
		operatorv1beta1.AddToScheme,
		certv1.AddToScheme,
		routev1.AddToScheme,
		consolev1.AddToScheme,
	)
	err := sb.AddToScheme(s)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return s
}

func NewTESTRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		certv1.SchemeGroupVersion,
	})
	// Add cert-manager Issuer GVK
	mapper.Add(schema.GroupVersionKind{
		Group:   certv1.SchemeGroupVersion.Group,
		Version: certv1.SchemeGroupVersion.Version,
		Kind:    certv1.IssuerKind,
	}, meta.RESTScopeNamespace)
	return mapper
}

func NewCryostat() *operatorv1beta1.Cryostat {
	certManager := true
	return &operatorv1beta1.Cryostat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
		},
		Spec: operatorv1beta1.CryostatSpec{
			Minimal:            false,
			EnableCertManager:  &certManager,
			TrustedCertSecrets: []operatorv1beta1.CertificateSecret{},
		},
	}
}

func NewCryostatWithSecrets() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	key := "test.crt"
	cr.Spec.TrustedCertSecrets = []operatorv1beta1.CertificateSecret{
		{
			SecretName:     "testCert1",
			CertificateKey: &key,
		},
		{
			SecretName: "testCert2",
		},
	}
	return cr
}

func NewCryostatWithTemplates() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.EventTemplates = []operatorv1beta1.TemplateConfigMap{
		{
			ConfigMapName: "templateCM1",
			Filename:      "template.jfc",
		},
		{
			ConfigMapName: "templateCM2",
			Filename:      "other-template.jfc",
		},
	}
	return cr
}

func NewCryostatWithIngress() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	networkConfig := NewNetworkConfigurationList()
	cr.Spec.NetworkOptions = &networkConfig
	return cr
}

func NewCryostatWithPVCSpec() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.StorageOptions = &operatorv1beta1.StorageConfiguration{
		PVC: &operatorv1beta1.PersistentVolumeClaimConfig{
			Annotations: map[string]string{
				"my/custom": "annotation",
			},
			Labels: map[string]string{
				"my":  "label",
				"app": "somethingelse",
			},
			Spec: newPVCSpec("cool-storage", "10Gi", corev1.ReadWriteMany),
		},
	}
	return cr
}

func NewCryostatWithPVCSpecSomeDefault() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.StorageOptions = &operatorv1beta1.StorageConfiguration{
		PVC: &operatorv1beta1.PersistentVolumeClaimConfig{
			Spec: newPVCSpec("", "1Gi"),
		},
	}
	return cr
}

func NewCryostatWithPVCLabelsOnly() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.StorageOptions = &operatorv1beta1.StorageConfiguration{
		PVC: &operatorv1beta1.PersistentVolumeClaimConfig{
			Labels: map[string]string{
				"my": "label",
			},
		},
	}
	return cr
}

func NewCryostatCertManagerDisabled() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	certManager := false
	cr.Spec.EnableCertManager = &certManager
	return cr
}

func NewCryostatCertManagerUndefined() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.EnableCertManager = nil
	return cr
}

func newPVCSpec(storageClass string, storageRequest string,
	accessModes ...corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaimSpec {
	return &corev1.PersistentVolumeClaimSpec{
		StorageClassName: &storageClass,
		AccessModes:      accessModes,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(storageRequest),
			},
		},
	}
}

func NewMinimalCryostat() *operatorv1beta1.Cryostat {
	cr := NewCryostat()
	cr.Spec.Minimal = true
	return cr
}

func NewFlightRecorder() *operatorv1beta1.FlightRecorder {
	return newFlightRecorder(&operatorv1beta1.JMXAuthSecret{
		SecretName: "test-jmx-auth",
	})
}

func NewFlightRecorderNoJMXAuth() *operatorv1beta1.FlightRecorder {
	return newFlightRecorder(nil)
}

func NewFlightRecorderBadJMXUserKey() *operatorv1beta1.FlightRecorder {
	key := "not-username"
	return newFlightRecorder(&operatorv1beta1.JMXAuthSecret{
		SecretName:  "test-jmx-auth",
		UsernameKey: &key,
	})
}

func NewFlightRecorderBadJMXPassKey() *operatorv1beta1.FlightRecorder {
	key := "not-password"
	return newFlightRecorder(&operatorv1beta1.JMXAuthSecret{
		SecretName:  "test-jmx-auth",
		PasswordKey: &key,
	})
}

func NewFlightRecorderForCryostat() *operatorv1beta1.FlightRecorder {
	userKey := "CRYOSTAT_RJMX_USER"
	passKey := "CRYOSTAT_RJMX_PASS"
	recorder := newFlightRecorder(&operatorv1beta1.JMXAuthSecret{
		SecretName:  "cryostat-jmx-auth",
		UsernameKey: &userKey,
		PasswordKey: &passKey,
	})
	recorder.Name = "cryostat-pod"
	recorder.Labels = map[string]string{"app": "cryostat-pod"}
	recorder.OwnerReferences[0].Name = "cryostat-pod"
	recorder.Spec.RecordingSelector.MatchLabels = map[string]string{"operator.cryostat.io/flightrecorder": "cryostat-pod"}
	return recorder
}

func newFlightRecorder(jmxAuth *operatorv1beta1.JMXAuthSecret) *operatorv1beta1.FlightRecorder {
	return &operatorv1beta1.FlightRecorder{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FlightRecorder",
			APIVersion: "operator.cryostat.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-pod",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test-pod",
					UID:        "",
				},
			},
		},
		Spec: operatorv1beta1.FlightRecorderSpec{
			JMXCredentials:    jmxAuth,
			RecordingSelector: metav1.AddLabelToSelector(&metav1.LabelSelector{}, operatorv1beta1.RecordingLabel, "test-pod"),
		},
		Status: operatorv1beta1.FlightRecorderStatus{
			Target: &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-pod",
				Namespace:  "default",
			},
			Port: 8001,
		},
	}
}

func NewRecording() *operatorv1beta1.Recording {
	return newRecording(getDuration(false), nil, nil, false)
}

func NewContinuousRecording() *operatorv1beta1.Recording {
	return newRecording(getDuration(true), nil, nil, false)
}

func NewRunningRecording() *operatorv1beta1.Recording {
	running := operatorv1beta1.RecordingStateRunning
	return newRecording(getDuration(false), &running, nil, false)
}

func NewRunningContinuousRecording() *operatorv1beta1.Recording {
	running := operatorv1beta1.RecordingStateRunning
	return newRecording(getDuration(true), &running, nil, false)
}

func NewRecordingToStop() *operatorv1beta1.Recording {
	running := operatorv1beta1.RecordingStateRunning
	stopped := operatorv1beta1.RecordingStateStopped
	return newRecording(getDuration(true), &running, &stopped, false)
}

func NewStoppedRecordingToArchive() *operatorv1beta1.Recording {
	stopped := operatorv1beta1.RecordingStateStopped
	return newRecording(getDuration(false), &stopped, nil, true)
}

func NewRecordingToStopAndArchive() *operatorv1beta1.Recording {
	running := operatorv1beta1.RecordingStateRunning
	stopped := operatorv1beta1.RecordingStateStopped
	return newRecording(getDuration(true), &running, &stopped, true)
}

func NewArchivedRecording() *operatorv1beta1.Recording {
	stopped := operatorv1beta1.RecordingStateStopped
	rec := newRecording(getDuration(false), &stopped, nil, true)
	savedDownloadURL := "http://path/to/saved-test-recording.jfr"
	savedReportURL := "http://path/to/saved-test-recording.html"
	rec.Status.DownloadURL = &savedDownloadURL
	rec.Status.ReportURL = &savedReportURL
	return rec
}

func NewDeletedArchivedRecording() *operatorv1beta1.Recording {
	rec := NewArchivedRecording()
	delTime := metav1.Unix(0, 1598045501618*int64(time.Millisecond))
	rec.DeletionTimestamp = &delTime
	return rec
}

func newRecording(duration time.Duration, currentState *operatorv1beta1.RecordingState,
	requestedState *operatorv1beta1.RecordingState, archive bool) *operatorv1beta1.Recording {
	finalizers := []string{}
	status := operatorv1beta1.RecordingStatus{}
	if currentState != nil {
		downloadUrl := "http://path/to/test-recording.jfr"
		reportUrl := "http://path/to/test-recording.html"
		finalizers = append(finalizers, "operator.cryostat.io/recording.finalizer")
		status = operatorv1beta1.RecordingStatus{
			State:       currentState,
			StartTime:   metav1.Unix(0, 1597090030341*int64(time.Millisecond)),
			Duration:    metav1.Duration{Duration: duration},
			DownloadURL: &downloadUrl,
			ReportURL:   &reportUrl,
		}
	}
	return &operatorv1beta1.Recording{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "my-recording",
			Namespace:  "default",
			Finalizers: finalizers,
		},
		Spec: operatorv1beta1.RecordingSpec{
			Name: "test-recording",
			EventOptions: []string{
				"jdk.socketRead:enabled=true",
				"jdk.socketWrite:enabled=true",
			},
			Duration: metav1.Duration{Duration: duration},
			State:    requestedState,
			Archive:  archive,
			FlightRecorder: &corev1.LocalObjectReference{
				Name: "test-pod",
			},
		},
		Status: status,
	}
}

func getDuration(continuous bool) time.Duration {
	seconds := 0
	if !continuous {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func NewTargetPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			PodIP: "1.2.3.4",
		},
	}
}

func NewCryostatPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			PodIP: "1.2.3.4",
		},
	}
}

func NewTestEndpoints() *corev1.Endpoints {
	target := &corev1.ObjectReference{
		Kind:      "Pod",
		Name:      "test-pod",
		Namespace: "default",
	}
	ports := []corev1.EndpointPort{
		{
			Name: "jfr-jmx",
			Port: 1234,
		},
		{
			Name: "other-port",
			Port: 9091,
		},
	}
	return newTestEndpoints(target, ports)
}

func NewTestEndpointsNoTargetRef() *corev1.Endpoints {
	ports := []corev1.EndpointPort{
		{
			Name: "jfr-jmx",
			Port: 1234,
		},
		{
			Name: "other-port",
			Port: 9091,
		},
	}
	return newTestEndpoints(nil, ports)
}

func NewTestEndpointsNoPorts() *corev1.Endpoints {
	target := &corev1.ObjectReference{
		Kind:      "Pod",
		Name:      "test-pod",
		Namespace: "default",
	}
	return newTestEndpoints(target, nil)
}

func NewTestEndpointsNoJMXPort() *corev1.Endpoints {
	target := &corev1.ObjectReference{
		Kind:      "Pod",
		Name:      "test-pod",
		Namespace: "default",
	}
	ports := []corev1.EndpointPort{
		{
			Name: "other-port",
			Port: 9091,
		},
	}
	return newTestEndpoints(target, ports)
}

func newTestEndpoints(targetRef *corev1.ObjectReference, ports []corev1.EndpointPort) *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:        "1.2.3.4",
						Hostname:  "test-pod",
						TargetRef: targetRef,
					},
				},
				Ports: ports,
			},
		},
	}
}

func NewCryostatEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:       "1.2.3.4",
						Hostname: "cryostat-pod",
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "cryostat-pod",
							Namespace: "default",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Name: "jfr-jmx",
						Port: 1234,
					},
				},
			},
		},
	}
}

func NewCryostatService() *corev1.Service {
	c := true
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: operatorv1beta1.GroupVersion.String(),
					Kind:       "Cryostat",
					Name:       "cryostat",
					UID:        "",
					Controller: &c,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Ports: []corev1.ServicePort{
				{
					Name: "export",
					Port: 8181,
				},
			},
		},
	}
}

func NewTestService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Ports: []corev1.ServicePort{
				{
					Name: "test",
					Port: 8181,
				},
			},
		},
	}
}

func NewCACert() *certv1.Certificate {
	return &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat-ca",
			Namespace: "default",
		},
		Spec: certv1.CertificateSpec{
			SecretName: "cryostat-ca",
		},
	}
}

func newCASecret(certData []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat-ca",
			Namespace: "default",
		},
		Data: map[string][]byte{
			corev1.TLSCertKey: certData,
		},
	}
}

func NewJMXAuthSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jmx-auth",
			Namespace: "default",
		},
		Data: map[string][]byte{
			operatorv1beta1.DefaultUsernameKey: []byte("hello"),
			operatorv1beta1.DefaultPasswordKey: []byte("world"),
		},
	}
}

func NewJMXAuthSecretForCryostat() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat-jmx-auth",
			Namespace: "default",
		},
		Data: map[string][]byte{
			operatorv1beta1.DefaultUsernameKey: []byte("hello"),
			operatorv1beta1.DefaultPasswordKey: []byte("world"),
		},
	}
}

func newPVC(spec *corev1.PersistentVolumeClaimSpec, labels map[string]string,
	annotations map[string]string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cryostat",
			Namespace:   "default",
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: *spec,
	}
}

func NewDefaultPVC() *corev1.PersistentVolumeClaim {
	return newPVC(&corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("500Mi"),
			},
		},
	}, map[string]string{
		"app": "cryostat",
	}, nil)
}

func NewCustomPVC() *corev1.PersistentVolumeClaim {
	storageClass := "cool-storage"
	return newPVC(&corev1.PersistentVolumeClaimSpec{
		StorageClassName: &storageClass,
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}, map[string]string{
		"my":  "label",
		"app": "cryostat",
	}, map[string]string{
		"my/custom": "annotation",
	})
}

func NewCustomPVCSomeDefault() *corev1.PersistentVolumeClaim {
	storageClass := ""
	return newPVC(&corev1.PersistentVolumeClaimSpec{
		StorageClassName: &storageClass,
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}, map[string]string{
		"app": "cryostat",
	}, nil)
}

func NewDefaultPVCWithLabel() *corev1.PersistentVolumeClaim {
	return newPVC(&corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("500Mi"),
			},
		},
	}, map[string]string{
		"app": "cryostat",
		"my":  "label",
	}, nil)
}

func NewCorePorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 8181,
		},
		{
			ContainerPort: 9090,
		},
		{
			ContainerPort: 9091,
		},
	}
}

func NewGrafanaPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 3000,
		},
	}
}

func NewDatasourcePorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 8080,
		},
	}
}

func NewCoreEnvironmentVariables(minimal bool, tls bool) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "CRYOSTAT_SSL_PROXIED",
			Value: "true",
		},
		{
			Name:  "CRYOSTAT_ALLOW_UNTRUSTED_SSL",
			Value: "true",
		},
		{
			Name:  "CRYOSTAT_WEB_PORT",
			Value: "8181",
		},
		{
			Name:  "CRYOSTAT_EXT_WEB_PORT",
			Value: "443",
		},
		{
			Name:  "CRYOSTAT_WEB_HOST",
			Value: "cryostat.example.com",
		},
		{
			Name:  "CRYOSTAT_LISTEN_PORT",
			Value: "9090",
		},
		{
			Name:  "CRYOSTAT_EXT_LISTEN_PORT",
			Value: "443",
		},
		{
			Name:  "CRYOSTAT_LISTEN_HOST",
			Value: "cryostat-command.example.com",
		},
		{
			Name:  "CRYOSTAT_CONFIG_PATH",
			Value: "/opt/cryostat.d/conf.d",
		},
		{
			Name:  "CRYOSTAT_ARCHIVE_PATH",
			Value: "/opt/cryostat.d/recordings.d",
		},
		{
			Name:  "CRYOSTAT_TEMPLATE_PATH",
			Value: "/opt/cryostat.d/templates.d",
		},
		{
			Name:  "CRYOSTAT_CLIENTLIB_PATH",
			Value: "/opt/cryostat.d/clientlib.d",
		},
	}
	if !minimal {
		envs = append(envs,
			corev1.EnvVar{
				Name:  "GRAFANA_DASHBOARD_URL",
				Value: "https://cryostat-grafana.example.com",
			},
			corev1.EnvVar{
				Name:  "GRAFANA_DATASOURCE_URL",
				Value: "http://127.0.0.1:8080",
			})
	}
	if !tls {
		envs = append(envs,
			corev1.EnvVar{
				Name:  "CRYOSTAT_DISABLE_SSL",
				Value: "true",
			})
	} else {
		envs = append(envs, corev1.EnvVar{
			Name:  "KEYSTORE_PATH",
			Value: "/var/run/secrets/operator.cryostat.io/cryostat-tls/keystore.p12",
		})
	}
	return envs
}

func NewGrafanaEnvironmentVariables(tls bool) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "JFR_DATASOURCE_URL",
			Value: "http://127.0.0.1:8080",
		},
	}
	if tls {
		envs = append(envs, corev1.EnvVar{
			Name:  "GF_SERVER_PROTOCOL",
			Value: "https",
		}, corev1.EnvVar{
			Name:  "GF_SERVER_CERT_KEY",
			Value: "/var/run/secrets/operator.cryostat.io/cryostat-grafana-tls/tls.key",
		}, corev1.EnvVar{
			Name:  "GF_SERVER_CERT_FILE",
			Value: "/var/run/secrets/operator.cryostat.io/cryostat-grafana-tls/tls.crt",
		})
	}
	return envs
}

func NewDatasourceEnvironmentVariables() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "LISTEN_HOST",
			Value: "127.0.0.1",
		},
	}
}

func NewCoreEnvFromSource(tls bool) []corev1.EnvFromSource {
	envsFrom := []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "cryostat-jmx-auth",
				},
			},
		},
	}
	if tls {
		envsFrom = append(envsFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "cryostat-keystore",
				},
			},
		})
	}
	return envsFrom
}

func NewGrafanaEnvFromSource() []corev1.EnvFromSource {
	return []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "cryostat-grafana-basic",
				},
			},
		},
	}
}

func NewCoreVolumeMounts(tls bool) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "cryostat",
			ReadOnly:  false,
			MountPath: "/opt/cryostat.d/conf.d",
			SubPath:   "config",
		},
		{
			Name:      "cryostat",
			ReadOnly:  false,
			MountPath: "/opt/cryostat.d/recordings.d",
			SubPath:   "flightrecordings",
		},
		{
			Name:      "cryostat",
			ReadOnly:  false,
			MountPath: "/opt/cryostat.d/templates.d",
			SubPath:   "templates",
		},
		{
			Name:      "cryostat",
			ReadOnly:  false,
			MountPath: "/opt/cryostat.d/clientlib.d",
			SubPath:   "clientlib",
		},
		{
			Name:      "cryostat",
			ReadOnly:  false,
			MountPath: "truststore",
			SubPath:   "truststore",
		},
	}
	if tls {
		mounts = append(mounts,
			corev1.VolumeMount{
				Name:      "keystore",
				ReadOnly:  true,
				MountPath: "/var/run/secrets/operator.cryostat.io/cryostat-tls",
			},
			corev1.VolumeMount{
				Name:      "cert-secrets",
				ReadOnly:  true,
				MountPath: "/truststore/operator",
			})
	}
	return mounts
}

func NewGrafanaVolumeMounts(tls bool) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{}
	if tls {
		mounts = append(mounts,
			corev1.VolumeMount{
				Name:      "grafana-tls-secret",
				MountPath: "/var/run/secrets/operator.cryostat.io/cryostat-grafana-tls",
				ReadOnly:  true,
			})
	}
	return mounts
}

func NewVolumeMountsWithTemplates() []corev1.VolumeMount {
	return append(NewCoreVolumeMounts(true),
		corev1.VolumeMount{
			Name:      "template-templateCM1",
			ReadOnly:  true,
			MountPath: "/opt/cryostat.d/templates.d/templateCM1_template.jfc",
			SubPath:   "template.jfc",
		},
		corev1.VolumeMount{
			Name:      "template-templateCM2",
			ReadOnly:  true,
			MountPath: "/opt/cryostat.d/templates.d/templateCM2_other-template.jfc",
			SubPath:   "other-template.jfc",
		})
}

func NewCoreLivenessProbe(tls bool) *corev1.Probe {
	return &corev1.Probe{
		Handler: newCoreProbeHandler(tls),
	}
}

func NewCoreStartupProbe(tls bool) *corev1.Probe {
	return &corev1.Probe{
		Handler:          newCoreProbeHandler(tls),
		FailureThreshold: 18,
	}
}

func newCoreProbeHandler(tls bool) corev1.Handler {
	protocol := corev1.URISchemeHTTPS
	if !tls {
		protocol = corev1.URISchemeHTTP
	}
	return corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Port:   intstr.IntOrString{IntVal: 8181},
			Path:   "/api/v1/clienturl",
			Scheme: protocol,
		},
	}
}

func NewGrafanaLivenessProbe(tls bool) *corev1.Probe {
	protocol := corev1.URISchemeHTTPS
	if !tls {
		protocol = corev1.URISchemeHTTP
	}
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port:   intstr.IntOrString{IntVal: 3000},
				Path:   "/api/health",
				Scheme: protocol,
			},
		},
	}
}

func NewDatasourceLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"curl", "--fail", "http://127.0.0.1:8080"},
			},
		},
	}
}

func NewDeploymentSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":  "cryostat",
			"kind": "cryostat",
		},
	}
}

func NewVolumes(minimal bool, tls bool) []corev1.Volume {
	return newVolumes(minimal, tls, nil)
}

func NewVolumesWithSecrets() []corev1.Volume {
	mode := int32(0440)
	return newVolumes(false, true, []corev1.VolumeProjection{
		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "testCert1",
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "test.crt",
						Path: "testCert1_test.crt",
						Mode: &mode,
					},
				},
			},
		},
		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "testCert2",
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "tls.crt",
						Path: "testCert2_tls.crt",
						Mode: &mode,
					},
				},
			},
		},
	})
}

func NewVolumesWithTemplates() []corev1.Volume {
	mode := int32(0440)
	return append(NewVolumes(false, true),
		corev1.Volume{
			Name: "template-templateCM1",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "templateCM1",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "template.jfc",
							Path: "template.jfc",
							Mode: &mode,
						},
					},
				},
			},
		},
		corev1.Volume{
			Name: "template-templateCM2",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "templateCM2",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "other-template.jfc",
							Path: "other-template.jfc",
							Mode: &mode,
						},
					},
				},
			},
		})
}

func newVolumes(minimal bool, tls bool, certProjections []corev1.VolumeProjection) []corev1.Volume {
	readOnlymode := int32(0440)
	volumes := []corev1.Volume{
		{
			Name: "cryostat",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "cryostat",
					ReadOnly:  false,
				},
			},
		},
	}
	if tls {
		projs := []corev1.VolumeProjection{
			{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cryostat-tls",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "ca.crt",
							Path: "cryostat-ca.crt",
							Mode: &readOnlymode,
						},
					},
				},
			},
		}
		projs = append(projs, certProjections...)
		volumes = append(volumes,
			corev1.Volume{
				Name: "cert-secrets",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: projs,
					},
				},
			})

		volumes = append(volumes,
			corev1.Volume{
				Name: "keystore",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "cryostat-tls",
						Items: []corev1.KeyToPath{
							{
								Key:  "keystore.p12",
								Path: "keystore.p12",
								Mode: &readOnlymode,
							},
						},
					},
				},
			})
		if !minimal {
			volumes = append(volumes,
				corev1.Volume{
					Name: "grafana-tls-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "cryostat-grafana-tls",
						},
					},
				})
		}
	}
	return volumes
}

func NewPodSecurityContext() *corev1.PodSecurityContext {
	fsGroup := int64(18500)
	return &corev1.PodSecurityContext{
		FSGroup: &fsGroup,
	}
}

func NewNetworkConfigurationList() operatorv1beta1.NetworkConfigurationList {
	coreSVC := resource_definitions.NewExporterService(NewCryostat())
	coreIng := NewNetworkConfiguration(coreSVC.Name, coreSVC.Spec.Ports[0].Port)

	commandSVC := resource_definitions.NewCommandChannelService(NewCryostat())
	commandIng := NewNetworkConfiguration(commandSVC.Name, commandSVC.Spec.Ports[0].Port)

	grafanaSVC := resource_definitions.NewGrafanaService(NewCryostat())
	grafanaIng := NewNetworkConfiguration(grafanaSVC.Name, grafanaSVC.Spec.Ports[0].Port)

	return operatorv1beta1.NetworkConfigurationList{
		CoreConfig:    &coreIng,
		CommandConfig: &commandIng,
		GrafanaConfig: &grafanaIng,
	}
}

func NewNetworkConfiguration(svcName string, svcPort int32) operatorv1beta1.NetworkConfiguration {
	pathtype := netv1.PathTypePrefix
	host := "testing." + svcName
	return operatorv1beta1.NetworkConfiguration{
		Annotations: map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS"},
		IngressSpec: &netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathtype,
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: svcName,
											Port: netv1.ServiceBackendPort{
												Number: svcPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func NewServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
		},
	}
}

func NewRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
			},
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"pods", "replicationcontrollers"},
			},
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets", "deployments", "daemonsets", "statefulsets"},
			},
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps.openshift.io"},
				Resources: []string{"deploymentconfigs"},
			},
			{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{"route.openshift.io"},
				Resources: []string{"routes"},
			},
		},
	}
}

func NewRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cryostat",
			Namespace: "default",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "cryostat",
				Namespace: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "cryostat",
		},
	}
}

func NewClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cryostat-9ecd5050500c2566765bc593edfcce12434283e5da32a27476bc4a1569304a02",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "cryostat",
				Namespace: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cryostat-operator-cryostat",
		},
	}
}

func NewTemplateConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "templateCM1",
			Namespace: "default",
		},
		Data: map[string]string{
			"template.jfc": "XML template data",
		},
	}
}

func NewOtherTemplateConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "templateCM2",
			Namespace: "default",
		},
		Data: map[string]string{
			"other-template.jfc": "more XML template data",
		},
	}
}

func NewNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
}

func NewNamespaceWithSCCSupGroups() *corev1.Namespace {
	ns := NewNamespace()
	ns.Annotations = map[string]string{
		securityv1.SupplementalGroupsAnnotation: "1000130000/10000",
	}
	return ns
}

func NewConsoleLink() *consolev1.ConsoleLink {
	return &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cryostat-9ecd5050500c2566765bc593edfcce12434283e5da32a27476bc4a1569304a02",
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Text: "Cryostat",
				Href: "https://cryostat.example.com",
			},
			Location: consolev1.NamespaceDashboard,
			NamespaceDashboard: &consolev1.NamespaceDashboardSpec{
				Namespaces: []string{"default"},
			},
		},
	}
}
