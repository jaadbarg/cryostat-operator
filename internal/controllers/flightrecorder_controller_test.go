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

package controllers_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1beta1 "github.com/cryostatio/cryostat-operator/api/v1beta1"
	"github.com/cryostatio/cryostat-operator/internal/controllers"
	"github.com/cryostatio/cryostat-operator/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type flightRecorderTestInput struct {
	controller *controllers.FlightRecorderReconciler
	objs       []runtime.Object
	handlers   []http.HandlerFunc
	test.TestReconcilerConfig
}

var _ = Describe("FlightRecorderController", func() {
	var t *flightRecorderTestInput

	JustBeforeEach(func() {
		logger := zap.New()
		logf.SetLogger(logger)
		s := test.NewTestScheme()

		t.Client = fake.NewFakeClientWithScheme(s, t.objs...)
		t.Server = test.NewServer(t.Client, t.handlers, t.TLS)
		t.controller = &controllers.FlightRecorderReconciler{
			Client:     t.Client,
			Scheme:     s,
			Log:        logger,
			Reconciler: test.NewTestReconciler(&t.TestReconcilerConfig),
		}
	})

	JustAfterEach(func() {
		t.Server.VerifyRequestsReceived(t.handlers)
		t.Server.Close()
	})

	BeforeEach(func() {
		t = &flightRecorderTestInput{
			objs: []runtime.Object{
				test.NewCryostat(), test.NewCACert(), test.NewFlightRecorder(), test.NewTargetPod(),
				test.NewCryostatService(), test.NewJMXAuthSecret(),
			},
			TestReconcilerConfig: test.TestReconcilerConfig{
				TLS: true,
			},
		}
	})

	AfterEach(func() {
		// Reset test inputs
		t = nil
	})

	Describe("reconciling a request", func() {
		Context("successfully updates FlightRecorder CR", func() {
			BeforeEach(func() {
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesHandler(),
					test.NewListTemplatesHandler(),
				}
			})
			It("should update event type list", func() {
				t.expectFlightRecorderReconcileSuccess()
			})
		})
		Context("after FlightRecorder already reconciled successfully", func() {
			BeforeEach(func() {
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesHandler(),
					test.NewListTemplatesHandler(),
					test.NewListEventTypesHandler(),
					test.NewListTemplatesHandler(),
				}
			})
			It("should be idempotent", func() {
				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pod", Namespace: "default"}}
				result, err := t.controller.Reconcile(context.Background(), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				obj := &operatorv1beta1.FlightRecorder{}
				err = t.Client.Get(context.Background(), req.NamespacedName, obj)
				Expect(err).ToNot(HaveOccurred())

				// Reconcile same FlightRecorder again
				result, err = t.controller.Reconcile(context.Background(), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				obj2 := &operatorv1beta1.FlightRecorder{}
				err = t.Client.Get(context.Background(), req.NamespacedName, obj2)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj2.Status).To(Equal(obj.Status))
				Expect(obj2.Spec).To(Equal(obj.Spec))
			})
		})
		Context("FlightRecorder does not exist", func() {
			It("should do nothing", func() {
				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "does-not-exist", Namespace: "default"}}
				result, err := t.controller.Reconcile(context.Background(), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
		Context("FlightRecorder Status not updated yet", func() {
			BeforeEach(func() {
				otherFr := test.NewFlightRecorder()
				otherFr.Status = operatorv1beta1.FlightRecorderStatus{}
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), otherFr, test.NewTargetPod(), test.NewCryostatService(),
					test.NewJMXAuthSecret(),
				}
			})
			It("should requeue", func() {
				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pod", Namespace: "default"}}
				result, err := t.controller.Reconcile(context.Background(), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: time.Second}))
			})
		})
		Context("list-event-types command fails", func() {
			BeforeEach(func() {
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesFailHandler(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("list-templates command fails", func() {
			BeforeEach(func() {
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesHandler(),
					test.NewListTemplatesFailHandler(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("Cryostat CR is missing", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewFlightRecorder(), test.NewCACert(), test.NewTargetPod(), test.NewCryostatService(),
					test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("Cryostat service is missing", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorder(), test.NewTargetPod(),
					test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("Target pod is missing", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorder(), test.NewCryostatService(),
					test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("Target pod has no IP", func() {
			BeforeEach(func() {
				otherPod := test.NewTargetPod()
				otherPod.Status.PodIP = ""
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorder(), otherPod, test.NewCryostatService(),
					test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("successfully updates FlightRecorder CR without JMX auth", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorderNoJMXAuth(),
					test.NewTargetPod(), test.NewCryostatService(),
				}
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesNoJMXAuthHandler(),
					test.NewListTemplatesNoJMXAuthHandler(),
				}
			})
			It("should update event type list and template list", func() {
				t.expectFlightRecorderReconcileSuccess()
			})
		})
		Context("incorrect key name for JMX auth secret", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorderBadJMXUserKey(),
					test.NewTargetPod(), test.NewCryostatService(), test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("incorrect password key name for JMX auth secret", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorderBadJMXPassKey(),
					test.NewTargetPod(), test.NewCryostatService(), test.NewJMXAuthSecret(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("missing JMX auth secret", func() {
			BeforeEach(func() {
				t.objs = []runtime.Object{
					test.NewCryostat(), test.NewCACert(), test.NewFlightRecorder(),
					test.NewTargetPod(), test.NewCryostatService(),
				}
			})
			It("should requeue with error", func() {
				t.expectFlightRecorderReconcileError()
			})
		})
		Context("successfully updates FlightRecorder CR with TLS disabled", func() {
			BeforeEach(func() {
				t.handlers = []http.HandlerFunc{
					test.NewListEventTypesHandler(),
					test.NewListTemplatesHandler(),
				}
				disableTLS := true
				t.EnvDisableTLS = &disableTLS
			})
			It("should update event type list", func() {
				t.expectFlightRecorderReconcileSuccess()
			})
		})
	})
})

func (t *flightRecorderTestInput) expectFlightRecorderReconcileSuccess() {
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pod", Namespace: "default"}}
	result, err := t.controller.Reconcile(context.Background(), req)
	Expect(err).ToNot(HaveOccurred())
	Expect(result).To(Equal(reconcile.Result{}))

	obj := &operatorv1beta1.FlightRecorder{}
	err = t.Client.Get(context.Background(), req.NamespacedName, obj)
	Expect(err).ToNot(HaveOccurred())
	Expect(obj.Status.Events).To(Equal(test.NewEventTypes()))
	Expect(obj.Status.Templates).To(Equal(test.NewTemplates()))
}

func (t *flightRecorderTestInput) expectFlightRecorderReconcileError() {
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pod", Namespace: "default"}}
	result, err := t.controller.Reconcile(context.Background(), req)
	Expect(err).To(HaveOccurred())
	Expect(result).To(Equal(reconcile.Result{}))
}
