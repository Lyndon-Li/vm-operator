// Copyright (c) 2024 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package vmopv1_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	vimtypes "github.com/vmware/govmomi/vim25/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	pkgconst "github.com/vmware-tanzu/vm-operator/pkg/constants"
	"github.com/vmware-tanzu/vm-operator/pkg/util/kube/cource"
	spqutil "github.com/vmware-tanzu/vm-operator/pkg/util/kube/spq"
	vmopv1util "github.com/vmware-tanzu/vm-operator/pkg/util/vmopv1"
	"github.com/vmware-tanzu/vm-operator/test/builder"
)

var _ = Describe("ErrImageNotFound", func() {
	It("should return true from apierrors.IsNotFound", func() {
		Expect(apierrors.IsNotFound(vmopv1util.ErrImageNotFound{})).To(BeTrue())
	})
})

var _ = Describe("ResolveImageName", func() {

	const (
		actualNamespace = "my-namespace"

		nsImg1ID   = "vmi-1"
		nsImg1Name = "image-a"

		nsImg2ID   = "vmi-2"
		nsImg2Name = "image-b"

		nsImg3ID   = "vmi-3"
		nsImg3Name = "image-b"

		nsImg4ID   = "vmi-4"
		nsImg4Name = "image-c"

		clImg1ID   = "vmi-5"
		clImg1Name = "image-d"

		clImg2ID   = "vmi-6"
		clImg2Name = "image-e"

		clImg3ID   = "vmi-7"
		clImg3Name = "image-e"

		clImg4ID   = "vmi-8"
		clImg4Name = "image-c"
	)

	var (
		name      string
		namespace string
		client    ctrlclient.Client
		err       error
		obj       ctrlclient.Object
	)

	BeforeEach(func() {
		namespace = actualNamespace

		newNsImgFn := func(id, name string) *vmopv1.VirtualMachineImage {
			img := builder.DummyVirtualMachineImage(id)
			img.Namespace = actualNamespace
			img.Status.Name = name
			return img
		}

		newClImgFn := func(id, name string) *vmopv1.ClusterVirtualMachineImage {
			img := builder.DummyClusterVirtualMachineImage(id)
			img.Status.Name = name
			return img
		}

		// Replace the client with a fake client that has the index of VM images.
		client = fake.NewClientBuilder().WithScheme(builder.NewScheme()).
			WithIndex(
				&vmopv1.VirtualMachineImage{},
				"status.name",
				func(rawObj ctrlclient.Object) []string {
					image := rawObj.(*vmopv1.VirtualMachineImage)
					return []string{image.Status.Name}
				}).
			WithIndex(&vmopv1.ClusterVirtualMachineImage{},
				"status.name",
				func(rawObj ctrlclient.Object) []string {
					image := rawObj.(*vmopv1.ClusterVirtualMachineImage)
					return []string{image.Status.Name}
				}).
			WithObjects(
				newNsImgFn(nsImg1ID, nsImg1Name),
				newNsImgFn(nsImg2ID, nsImg2Name),
				newNsImgFn(nsImg3ID, nsImg3Name),
				newNsImgFn(nsImg4ID, nsImg4Name),
				newClImgFn(clImg1ID, clImg1Name),
				newClImgFn(clImg2ID, clImg2Name),
				newClImgFn(clImg3ID, clImg3Name),
				newClImgFn(clImg4ID, clImg4Name),
			).
			Build()
	})

	JustBeforeEach(func() {
		obj, err = vmopv1util.ResolveImageName(
			context.Background(), client, namespace, name)
	})

	When("name is vmi", func() {
		When("no image exists", func() {
			const missingVmi = "vmi-9999999"
			BeforeEach(func() {
				name = missingVmi
			})
			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(err).To(BeAssignableToTypeOf(vmopv1util.ErrImageNotFound{}))
				Expect(err.Error()).To(Equal(fmt.Sprintf("no VM image exists for %q in namespace or cluster scope", missingVmi)))
				Expect(obj).To(BeNil())
			})
		})
		When("img is namespace-scoped", func() {
			BeforeEach(func() {
				name = nsImg1ID
			})
			It("should return image ref", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(obj).To(BeAssignableToTypeOf(&vmopv1.VirtualMachineImage{}))
				img := obj.(*vmopv1.VirtualMachineImage)
				Expect(img.Name).To(Equal(nsImg1ID))
			})
		})
		When("img is cluster-scoped", func() {
			BeforeEach(func() {
				name = clImg1ID
			})
			It("should return image ref", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(obj).To(BeAssignableToTypeOf(&vmopv1.ClusterVirtualMachineImage{}))
				img := obj.(*vmopv1.ClusterVirtualMachineImage)
				Expect(img.Name).To(Equal(clImg1ID))
			})
		})
	})

	When("name is display name", func() {
		BeforeEach(func() {
			name = nsImg1Name
		})
		It("should return image ref", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(BeAssignableToTypeOf(&vmopv1.VirtualMachineImage{}))
			img := obj.(*vmopv1.VirtualMachineImage)
			Expect(img.Name).To(Equal(nsImg1ID))
		})
	})
	When("name is empty", func() {
		BeforeEach(func() {
			name = ""
		})
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("imgName is empty"))
			Expect(obj).To(BeNil())
		})
	})

	When("name matches multiple, namespaced-scoped images", func() {
		BeforeEach(func() {
			name = nsImg2Name
		})
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("multiple VM images exist for %q in namespace scope", nsImg2Name)))
			Expect(obj).To(BeNil())
		})
	})

	When("name matches multiple, cluster-scoped images", func() {
		BeforeEach(func() {
			name = clImg2Name
		})
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("multiple VM images exist for %q in cluster scope", clImg2Name)))
			Expect(obj).To(BeNil())
		})
	})

	When("name matches both namespace and cluster-scoped images", func() {
		BeforeEach(func() {
			name = clImg4Name
		})
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("multiple VM images exist for %q in namespace and cluster scope", clImg4Name)))
			Expect(obj).To(BeNil())
		})
	})

	When("name does not match any namespace or cluster-scoped images", func() {
		const invalidImageID = "invalid"
		BeforeEach(func() {
			name = invalidImageID
		})
		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(err).To(BeAssignableToTypeOf(vmopv1util.ErrImageNotFound{}))
			Expect(err.Error()).To(Equal(fmt.Sprintf("no VM image exists for %q in namespace or cluster scope", invalidImageID)))
			Expect(obj).To(BeNil())
		})
	})

	When("name matches a single namespace-scoped image", func() {
		BeforeEach(func() {
			name = nsImg1Name
		})
		It("should return image ref", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(BeAssignableToTypeOf(&vmopv1.VirtualMachineImage{}))
			img := obj.(*vmopv1.VirtualMachineImage)
			Expect(img.Name).To(Equal(nsImg1ID))
		})
	})

	When("name matches a single cluster-scoped image", func() {
		BeforeEach(func() {
			name = clImg1Name
		})
		It("should return image ref", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(BeAssignableToTypeOf(&vmopv1.ClusterVirtualMachineImage{}))
			img := obj.(*vmopv1.ClusterVirtualMachineImage)
			Expect(img.Name).To(Equal(clImg1ID))
		})
	})
})

var _ = DescribeTable("DetermineHardwareVersion",
	func(
		vm vmopv1.VirtualMachine,
		configSpec vimtypes.VirtualMachineConfigSpec,
		imgStatus vmopv1.VirtualMachineImageStatus,
		expected vimtypes.HardwareVersion,
	) {
		Ω(vmopv1util.DetermineHardwareVersion(vm, configSpec, imgStatus)).Should(Equal(expected))
	},
	Entry(
		"empty inputs",
		vmopv1.VirtualMachine{},
		vimtypes.VirtualMachineConfigSpec{},
		vmopv1.VirtualMachineImageStatus{},
		vimtypes.HardwareVersion(0),
	),
	Entry(
		"spec.minHardwareVersion is 11",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
			},
		},
		vimtypes.VirtualMachineConfigSpec{},
		vmopv1.VirtualMachineImageStatus{},
		vimtypes.HardwareVersion(11),
	),
	Entry(
		"spec.minHardwareVersion is 11, configSpec.version is 13",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
			},
		},
		vimtypes.VirtualMachineConfigSpec{
			Version: "vmx-13",
		},
		vmopv1.VirtualMachineImageStatus{},
		vimtypes.HardwareVersion(13),
	),
	Entry(
		"spec.minHardwareVersion is 11, configSpec.version is invalid",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
			},
		},
		vimtypes.VirtualMachineConfigSpec{
			Version: "invalid",
		},
		vmopv1.VirtualMachineImageStatus{},
		vimtypes.HardwareVersion(11),
	),
	Entry(
		"spec.minHardwareVersion is 11, configSpec has pci pass-through",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
			},
		},
		vimtypes.VirtualMachineConfigSpec{
			DeviceChange: []vimtypes.BaseVirtualDeviceConfigSpec{
				&vimtypes.VirtualDeviceConfigSpec{
					Device: &vimtypes.VirtualPCIPassthrough{},
				},
			},
		},
		vmopv1.VirtualMachineImageStatus{},
		pkgconst.MinSupportedHWVersionForPCIPassthruDevices,
	),
	Entry(
		"spec.minHardwareVersion is 11, vm has pvc",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
				Volumes: []vmopv1.VirtualMachineVolume{
					{
						VirtualMachineVolumeSource: vmopv1.VirtualMachineVolumeSource{
							PersistentVolumeClaim: &vmopv1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
		},
		vimtypes.VirtualMachineConfigSpec{},
		vmopv1.VirtualMachineImageStatus{},
		pkgconst.MinSupportedHWVersionForPVC,
	),
	Entry(
		"spec.minHardwareVersion is 11, configSpec has pci pass-through, image version is 20",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
			},
		},
		vimtypes.VirtualMachineConfigSpec{
			DeviceChange: []vimtypes.BaseVirtualDeviceConfigSpec{
				&vimtypes.VirtualDeviceConfigSpec{
					Device: &vimtypes.VirtualPCIPassthrough{},
				},
			},
		},
		vmopv1.VirtualMachineImageStatus{
			HardwareVersion: &[]int32{20}[0],
		},
		vimtypes.HardwareVersion(20),
	),
	Entry(
		"spec.minHardwareVersion is 11, vm has pvc, image version is 20",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				MinHardwareVersion: 11,
				Volumes: []vmopv1.VirtualMachineVolume{
					{
						VirtualMachineVolumeSource: vmopv1.VirtualMachineVolumeSource{
							PersistentVolumeClaim: &vmopv1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
		},
		vimtypes.VirtualMachineConfigSpec{},
		vmopv1.VirtualMachineImageStatus{
			HardwareVersion: &[]int32{20}[0],
		},
		vimtypes.HardwareVersion(20),
	),
)

var _ = DescribeTable("HasPVC",
	func(
		vm vmopv1.VirtualMachine,
		expected bool,
	) {
		Ω(vmopv1util.HasPVC(vm)).Should(Equal(expected))
	},
	Entry(
		"spec.volumes is empty",
		vmopv1.VirtualMachine{},
		false,
	),
	Entry(
		"spec.volumes is non-empty with no pvcs",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Volumes: []vmopv1.VirtualMachineVolume{
					{
						Name: "hello",
					},
				},
			},
		},
		false,
	),
	Entry(
		"spec.volumes is non-empty with at least one pvc",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Volumes: []vmopv1.VirtualMachineVolume{
					{
						Name: "hello",
					},
					{
						Name: "world",
						VirtualMachineVolumeSource: vmopv1.VirtualMachineVolumeSource{
							PersistentVolumeClaim: &vmopv1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
		},
		true,
	),
)

var _ = DescribeTable("IsClasslessVM",
	func(
		vm vmopv1.VirtualMachine,
		expected bool,
	) {
		Ω(vmopv1util.IsClasslessVM(vm)).Should(Equal(expected))
	},
	Entry(
		"spec.className is empty",
		vmopv1.VirtualMachine{},
		true,
	),
	Entry(
		"spec.className is non-empty",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				ClassName: "small",
			},
		},
		false,
	),
)

var _ = DescribeTable("IsImageLessVM",
	func(
		vm vmopv1.VirtualMachine,
		expected bool,
	) {
		Ω(vmopv1util.IsImagelessVM(vm)).Should(Equal(expected))
	},
	Entry(
		"spec.image is nil and spec.imageName is empty",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Image:     nil,
				ImageName: "",
			},
		},
		true,
	),
	Entry(
		"spec.image is not nil and spec.imageName is empty",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Image:     &vmopv1.VirtualMachineImageRef{},
				ImageName: "",
			},
		},
		false,
	),
	Entry(
		"spec.image is nil and spec.imageName is non-empty",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Image:     nil,
				ImageName: "non-empty",
			},
		},
		false,
	),
	Entry(
		"spec.image is not nil and spec.imageName is non-empty",
		vmopv1.VirtualMachine{
			Spec: vmopv1.VirtualMachineSpec{
				Image:     &vmopv1.VirtualMachineImageRef{},
				ImageName: "non-empty",
			},
		},
		false,
	),
)

var _ = Describe("SyncStorageUsageForNamespace", func() {
	var (
		ctx          context.Context
		namespace    string
		storageClass string
		chanEvent    chan event.GenericEvent
	)
	BeforeEach(func() {
		ctx = cource.NewContext()
		namespace = "my-namespace"
		storageClass = "my-storage-class"
		chanEvent = spqutil.FromContext(ctx)
	})
	JustBeforeEach(func() {
		vmopv1util.SyncStorageUsageForNamespace(ctx, namespace, storageClass)
	})
	When("namespace is empty", func() {
		BeforeEach(func() {
			namespace = ""
		})
		Specify("no event should be received", func() {
			Consistently(chanEvent).ShouldNot(Receive())
		})
	})
	When("storageClassName is empty", func() {
		BeforeEach(func() {
			storageClass = ""
		})
		Specify("no event should be received", func() {
			Consistently(chanEvent).ShouldNot(Receive())
		})
	})
	When("namespace and storageClassName are both non-empty", func() {
		Specify("an event should be received", func() {
			e := <-chanEvent
			Expect(e).ToNot(BeNil())
			Expect(e.Object).ToNot(BeNil())
			Expect(e.Object.GetNamespace()).To(Equal(namespace))
			Expect(e.Object.GetName()).To(Equal(storageClass))
		})
	})
})
