// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package vmlifecycle

import (
	"context"
	"fmt"

	vimtypes "github.com/vmware/govmomi/vim25/types"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	"github.com/vmware-tanzu/vm-operator/pkg/providers/vsphere/network"
)

func BootStrapLinuxPrep(
	ctx context.Context,
	config *vimtypes.VirtualMachineConfigInfo,
	linuxPrepSpec *vmopv1.VirtualMachineBootstrapLinuxPrepSpec,
	vAppConfigSpec *vmopv1.VirtualMachineBootstrapVAppConfigSpec,
	bsArgs *BootstrapArgs) (*vimtypes.VirtualMachineConfigSpec, *vimtypes.CustomizationSpec, error) {

	nicSettingMap, err := network.GuestOSCustomization(bsArgs.NetworkResults)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GOSC NIC mappings: %w", err)
	}

	customSpec := &vimtypes.CustomizationSpec{
		Identity: &vimtypes.CustomizationLinuxPrep{
			HostName: &vimtypes.CustomizationFixedName{
				Name: bsArgs.HostName,
			},
			Domain:     bsArgs.DomainName,
			TimeZone:   linuxPrepSpec.TimeZone,
			HwClockUTC: linuxPrepSpec.HardwareClockIsUTC,
		},
		GlobalIPSettings: vimtypes.CustomizationGlobalIPSettings{
			DnsSuffixList: bsArgs.SearchSuffixes,
			DnsServerList: bsArgs.DNSServers,
		},
		NicSettingMap: nicSettingMap,
	}

	var configSpec *vimtypes.VirtualMachineConfigSpec
	if vAppConfigSpec != nil {
		configSpec = &vimtypes.VirtualMachineConfigSpec{}
		configSpec.VAppConfig, err = GetOVFVAppConfigForConfigSpec(
			config,
			vAppConfigSpec,
			bsArgs.BootstrapData.VAppData,
			bsArgs.BootstrapData.VAppExData,
			bsArgs.TemplateRenderFn)
	}

	return configSpec, customSpec, err
}
