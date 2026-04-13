//nolint:testpackage
package maas

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	maasv1alpha1 "github.com/opendatahub-io/models-as-a-service/maas-controller/api/maas/v1alpha1"

	. "github.com/onsi/gomega"
)

func maasTenantTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(maasv1alpha1.AddToScheme(s))
	utilruntime.Must(gwapiv1.Install(s))
	return s
}

func TestMaaSTenantReconcile_DeletionRemovesFinalizerAfterOwnedConfigMapDeleted(t *testing.T) {
	g := NewWithT(t)
	s := maasTenantTestScheme(t)

	now := metav1.NewTime(time.Now())
	tenant := &maasv1alpha1.MaaSTenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              maasv1alpha1.MaaSTenantInstanceName,
			UID:               types.UID("tenant-uid"),
			DeletionTimestamp: &now,
			Finalizers:        []string{maasTenantFinalizer},
		},
	}
	trueRef := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maas-owned",
			Namespace: "opendatahub",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         maasv1alpha1.GroupVersion.String(),
				Kind:               maasv1alpha1.MaaSTenantKind,
				Name:               tenant.Name,
				UID:                tenant.UID,
				Controller:         &trueRef,
				BlockOwnerDeletion: &trueRef,
			}},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(&maasv1alpha1.MaaSTenant{}).
		WithObjects(tenant, cm).
		Build()

	r := &MaaSTenantReconciler{
		Client:             cl,
		Scheme:             s,
		OperatorNamespace:  "opendatahub",
	}

	res1, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res1.RequeueAfter).To(Equal(finalizeRequeueInterval), "first pass issues child deletes and requeues")

	res2, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res2.RequeueAfter).To(BeNumerically("==", 0))

	var updated maasv1alpha1.MaaSTenant
	err = cl.Get(context.Background(), client.ObjectKey{Name: tenant.Name}, &updated)
	if apierrors.IsNotFound(err) {
		// Fake client may remove the tenant once the finalizer is gone while deletionTimestamp is set.
	} else {
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(updated.Finalizers).NotTo(ContainElement(maasTenantFinalizer))
	}

	var cms corev1.ConfigMapList
	g.Expect(cl.List(context.Background(), &cms, client.InNamespace("opendatahub"))).To(Succeed())
	g.Expect(cms.Items).To(BeEmpty())
}

func TestMaaSTenantReconcile_DeletionRequeuesWhileOwnedChildTerminating(t *testing.T) {
	g := NewWithT(t)
	s := maasTenantTestScheme(t)

	now := metav1.NewTime(time.Now())
	tenant := &maasv1alpha1.MaaSTenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              maasv1alpha1.MaaSTenantInstanceName,
			UID:               types.UID("tenant-uid"),
			DeletionTimestamp: &now,
			Finalizers:        []string{maasTenantFinalizer},
		},
	}
	trueRef := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maas-owned",
			Namespace: "opendatahub",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         maasv1alpha1.GroupVersion.String(),
				Kind:               maasv1alpha1.MaaSTenantKind,
				Name:               tenant.Name,
				UID:                tenant.UID,
				Controller:         &trueRef,
				BlockOwnerDeletion: &trueRef,
			}},
			DeletionTimestamp: &now,
			Finalizers:          []string{"test-finalizer"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(&maasv1alpha1.MaaSTenant{}).
		WithObjects(tenant, cm).
		Build()

	r := &MaaSTenantReconciler{
		Client:            cl,
		Scheme:            s,
		OperatorNamespace: "opendatahub",
	}

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.RequeueAfter).To(Equal(finalizeRequeueInterval))

	var updated maasv1alpha1.MaaSTenant
	g.Expect(cl.Get(context.Background(), client.ObjectKey{Name: tenant.Name}, &updated)).To(Succeed())
	g.Expect(updated.Finalizers).To(ContainElement(maasTenantFinalizer))
}
