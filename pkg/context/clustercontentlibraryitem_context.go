// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	imgregv1a1 "github.com/vmware-tanzu/image-registry-operator-api/api/v1alpha1"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
)

// ClusterContentLibraryItemContext is the context used for ClusterContentLibraryItem controller.
type ClusterContentLibraryItemContext struct {
	context.Context
	Logger       logr.Logger
	CCLItem      *imgregv1a1.ClusterContentLibraryItem
	CVMI         *vmopv1.ClusterVirtualMachineImage
	ImageObjName string
}

func (c *ClusterContentLibraryItemContext) String() string {
	return fmt.Sprintf("%s %s", c.CCLItem.GroupVersionKind(), c.CCLItem.Name)
}
