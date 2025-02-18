package awsnode

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ekscontrolplanev1 "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
)

func TestReconcileCniVpcCniValues(t *testing.T) {
	tests := []struct {
		name       string
		cniValues  ekscontrolplanev1.VpcCni
		daemonSet  *v1.DaemonSet
		consistsOf []corev1.EnvVar
	}{
		{
			name: "users can set environment values",
			cniValues: ekscontrolplanev1.VpcCni{
				Env: []corev1.EnvVar{
					{
						Name:  "NAME1",
						Value: "VALUE1",
					},
				},
			},
			daemonSet: &v1.DaemonSet{
				TypeMeta: metav1.TypeMeta{
					Kind: "DaemonSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-node",
					Namespace: "kube-system",
				},
				Spec: v1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "aws-node",
									Env:  []corev1.EnvVar{},
								},
							},
						},
					},
				},
			},
			consistsOf: []corev1.EnvVar{
				{
					Name:  "NAME1",
					Value: "VALUE1",
				},
			},
		},
		{
			name: "users can set environment values without duplications",
			cniValues: ekscontrolplanev1.VpcCni{
				Env: []corev1.EnvVar{
					{
						Name:  "NAME1",
						Value: "VALUE1",
					},
					{
						Name:  "NAME1",
						Value: "VALUE2",
					},
				},
			},
			daemonSet: &v1.DaemonSet{
				TypeMeta: metav1.TypeMeta{
					Kind: "DaemonSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-node",
					Namespace: "kube-system",
				},
				Spec: v1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "aws-node",
									Env:  []corev1.EnvVar{},
								},
							},
						},
					},
				},
			},
			consistsOf: []corev1.EnvVar{
				{
					Name:  "NAME1",
					Value: "VALUE2",
				},
			},
		},
		{
			name: "users can set environment values overwriting existing values",
			cniValues: ekscontrolplanev1.VpcCni{
				Env: []corev1.EnvVar{
					{
						Name:  "NAME1",
						Value: "VALUE1",
					},
					{
						Name:  "NAME2",
						Value: "VALUE2",
					},
				},
			},
			daemonSet: &v1.DaemonSet{
				TypeMeta: metav1.TypeMeta{
					Kind: "DaemonSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-node",
					Namespace: "kube-system",
				},
				Spec: v1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "aws-node",
									Env: []corev1.EnvVar{
										{
											Name:  "NAME1",
											Value: "OVERWRITE",
										},
										{
											Name:  "NAME3",
											Value: "VALUE3",
										},
									},
								},
							},
						},
					},
				},
			},
			consistsOf: []corev1.EnvVar{
				{
					Name:  "NAME1",
					Value: "VALUE1",
				},
				{
					Name:  "NAME2",
					Value: "VALUE2",
				},
				{
					Name:  "NAME3",
					Value: "VALUE3",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			g := NewWithT(t)
			mockClient := &cachingClient{
				getValue: tc.daemonSet,
			}
			m := &mockScope{
				client: mockClient,
				cni:    tc.cniValues,
			}
			s := NewService(m)

			err := s.ReconcileCNI(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(mockClient.updateChain).NotTo(BeEmpty())
			ds, ok := mockClient.updateChain[0].(*v1.DaemonSet)
			g.Expect(ok).To(BeTrue())
			g.Expect(ds.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			g.Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ConsistOf(tc.consistsOf))
		})
	}
}

type cachingClient struct {
	client.Client
	getValue    client.Object
	updateChain []client.Object
}

func (c *cachingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if _, ok := obj.(*v1.DaemonSet); ok {
		daemonset, _ := obj.(*v1.DaemonSet)
		*daemonset = *c.getValue.(*v1.DaemonSet)
	}
	return nil
}

func (c *cachingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateChain = append(c.updateChain, obj)
	return nil
}

type mockScope struct {
	scope.AWSNodeScope
	client client.Client
	cni    ekscontrolplanev1.VpcCni
}

func (s *mockScope) RemoteClient() (client.Client, error) {
	return s.client, nil
}

func (s *mockScope) VpcCni() ekscontrolplanev1.VpcCni {
	return s.cni
}

func (s *mockScope) Info(msg string, keysAndValues ...interface{}) {

}

func (s *mockScope) Name() string {
	return "mock-name"
}

func (s *mockScope) Namespace() string {
	return "mock-namespace"
}

func (s *mockScope) DisableVPCCNI() bool {
	return false
}

func (s *mockScope) SecondaryCidrBlock() *string {
	return nil
}
