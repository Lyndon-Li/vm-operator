// Copyright (c) 2018-2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	pkgcfg "github.com/vmware-tanzu/vm-operator/pkg/config"
	"github.com/vmware-tanzu/vm-operator/pkg/providers/vsphere/credentials"
)

var log = logf.Log.WithName("vsphere").WithName("config")

// VSphereVMProviderConfig represents the configuration for a Vsphere VM Provider instance.
// Contains information enabling integration with a backend vSphere instance for VM management.
type VSphereVMProviderConfig struct {
	VcPNID                      string
	VcPort                      string
	VcCreds                     *credentials.VSphereVMProviderCredentials
	Datacenter                  string
	StorageClassRequired        bool // Always true in WCP env.
	UseInventoryAsContentSource bool // Always false in WCP env.
	CAFilePath                  string
	InsecureSkipTLSVerify       bool // Always false in WCP env.

	// These are Zone and/or Namespace specific.
	ResourcePool string
	Folder       string

	// Only set in simulated testing env.
	Datastore string
	Network   string
}

const (
	DefaultVCPort = "443"

	ProviderConfigMapName = "vsphere.provider.config.vmoperator.vmware.com"
	// Keys in provider ConfigMap.
	vcPNIDKey                = "VcPNID"
	vcPortKey                = "VcPort"
	vcCredsSecretNameKey     = "VcCredsSecretName" //nolint:gosec
	datacenterKey            = "Datacenter"
	resourcePoolKey          = "ResourcePool"
	folderKey                = "Folder"
	datastoreKey             = "Datastore"
	networkNameKey           = "Network"
	scRequiredKey            = "StorageClassRequired"
	useInventoryKey          = "UseInventoryAsContentSource"
	insecureSkipTLSVerifyKey = "InsecureSkipTLSVerify"
	caFilePathKey            = "CAFilePath"

	NetworkConfigMapName = "vmoperator-network-config"
	NameserversKey       = "nameservers"    // Key in the NetworkConfigMapName.
	SearchSuffixesKey    = "searchsuffixes" // Key in the NetworkConfigMapName.
)

// ConfigMapToProviderConfig converts the VM provider ConfigMap to a VSphereVMProviderConfig.
func ConfigMapToProviderConfig( //nolint: revive // Ignore linter error about stuttering.
	configMap *corev1.ConfigMap,
	vcCreds *credentials.VSphereVMProviderCredentials) (*VSphereVMProviderConfig, error) {

	vcPNID, ok := configMap.Data[vcPNIDKey]
	if !ok {
		return nil, errors.New("missing configMap data field VcPNID")
	}

	vcPort, ok := configMap.Data[vcPortKey]
	if !ok {
		vcPort = DefaultVCPort
	}

	scRequired := false
	if s, ok := configMap.Data[scRequiredKey]; ok {
		var err error
		scRequired, err = strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value of StorageClassRequired: %w", err)
		}
	}

	useInventory := false
	if u, ok := configMap.Data[useInventoryKey]; ok {
		var err error
		useInventory, err = strconv.ParseBool(u)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value of UseInventory: %w", err)
		}
	}

	// Default to validating TLS.
	insecureSkipTLSVerify := false
	if v, ok := configMap.Data[insecureSkipTLSVerifyKey]; ok {
		var err error
		insecureSkipTLSVerify, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value of InsecureSkipTLSVerify: %w", err)
		}
	}

	var caFilePath string
	if ca, ok := configMap.Data[caFilePathKey]; !insecureSkipTLSVerify && ok {
		// The value will be /etc/vmware/wcp/tls/vmca.pem. While this is from our provider ConfigMap
		// it must match the volume path in our Deployment.
		caFilePath = ca
	}

	ret := &VSphereVMProviderConfig{
		VcPNID:                      vcPNID,
		VcPort:                      vcPort,
		VcCreds:                     vcCreds,
		Datacenter:                  configMap.Data[datacenterKey],
		ResourcePool:                configMap.Data[resourcePoolKey],
		Folder:                      configMap.Data[folderKey],
		Datastore:                   configMap.Data[datastoreKey],
		Network:                     configMap.Data[networkNameKey],
		StorageClassRequired:        scRequired,
		UseInventoryAsContentSource: useInventory,
		InsecureSkipTLSVerify:       insecureSkipTLSVerify,
		CAFilePath:                  caFilePath,
	}

	return ret, nil
}

func configMapToProviderCredentials(
	client ctrlclient.Client,
	configMap *corev1.ConfigMap) (*credentials.VSphereVMProviderCredentials, error) {

	secretName := configMap.Data[vcCredsSecretNameKey]
	if secretName == "" {
		return nil, fmt.Errorf("%s creds secret not set in vmop system namespace", vcCredsSecretNameKey)
	}

	return credentials.GetProviderCredentials(client, configMap.Namespace, secretName)
}

func GetDNSInformationFromConfigMap(ctx context.Context, client ctrlclient.Client) ([]string, []string, error) {
	vmopNamespace := pkgcfg.FromContext(ctx).PodNamespace

	configMap := &corev1.ConfigMap{}
	configMapKey := ctrlclient.ObjectKey{Name: NetworkConfigMapName, Namespace: vmopNamespace}
	if err := client.Get(ctx, configMapKey, configMap); err != nil {
		return nil, nil, err
	}

	var (
		nameservers    []string
		searchSuffixes []string
	)

	if nsStr, ok := configMap.Data[NameserversKey]; ok {
		nameservers = strings.Fields(nsStr)

		if len(nameservers) == 1 && nameservers[0] == "<worker_dns>" {
			return nil, nil, fmt.Errorf("no valid nameservers in %v ConfigMap. It still contains <worker_dns> key", NetworkConfigMapName)
		}
	}

	if ssStr, ok := configMap.Data[SearchSuffixesKey]; ok {
		searchSuffixes = strings.Fields(ssStr)
	}

	return nameservers, searchSuffixes, nil
}

// getProviderConfigMap returns the provider ConfigMap.
func getProviderConfigMap(
	ctx context.Context,
	client ctrlclient.Client) (*corev1.ConfigMap, error) {

	vmopNamespace := pkgcfg.FromContext(ctx).PodNamespace

	configMap := &corev1.ConfigMap{}
	configMapKey := ctrlclient.ObjectKey{Name: ProviderConfigMapName, Namespace: vmopNamespace}
	if err := client.Get(ctx, configMapKey, configMap); err != nil {
		// Log message used by VMC LINT. Refer to before making changes
		return nil, fmt.Errorf("error retrieving the provider ConfigMap %s: %w", configMapKey, err)
	}

	return configMap, nil
}

// GetProviderConfig returns a provider config constructed from vSphere Provider ConfigMap in the VM Operator namespace.
func GetProviderConfig(
	ctx context.Context,
	client ctrlclient.Client) (*VSphereVMProviderConfig, error) {

	configMap, err := getProviderConfigMap(ctx, client)
	if err != nil {
		return nil, err
	}

	vcCreds, err := configMapToProviderCredentials(client, configMap)
	if err != nil {
		return nil, err
	}

	providerConfig, err := ConfigMapToProviderConfig(configMap, vcCreds)
	if err != nil {
		return nil, err
	}

	return providerConfig, nil
}

func setConfigMapData(configMap *corev1.ConfigMap, config *VSphereVMProviderConfig, vcCredsSecretName string) {
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}

	configMap.Data[vcPNIDKey] = config.VcPNID
	configMap.Data[vcPortKey] = config.VcPort
	configMap.Data[vcCredsSecretNameKey] = vcCredsSecretName
	configMap.Data[datacenterKey] = config.Datacenter
	configMap.Data[resourcePoolKey] = config.ResourcePool
	configMap.Data[folderKey] = config.Folder
	configMap.Data[datastoreKey] = config.Datastore
	configMap.Data[scRequiredKey] = strconv.FormatBool(config.StorageClassRequired)
	configMap.Data[useInventoryKey] = strconv.FormatBool(config.UseInventoryAsContentSource)
	configMap.Data[caFilePathKey] = config.CAFilePath
	configMap.Data[insecureSkipTLSVerifyKey] = strconv.FormatBool(config.InsecureSkipTLSVerify)
}

// ProviderConfigToConfigMap returns the ConfigMap for the config.
// Used only in testing.
func ProviderConfigToConfigMap(
	namespace string,
	config *VSphereVMProviderConfig,
	vcCredsSecretName string) *corev1.ConfigMap {

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProviderConfigMapName,
			Namespace: namespace,
		},
	}
	setConfigMapData(configMap, config, vcCredsSecretName)

	return configMap
}

// UpdateVcInConfigMap updates the ConfigMap with the new vCenter PNID and Port. Returns false if no updated needed.
func UpdateVcInConfigMap(ctx context.Context, client ctrlclient.Client, vcPNID, vcPort string) (bool, error) {
	configMap, err := getProviderConfigMap(ctx, client)
	if err != nil {
		return false, err
	}

	if configMap.Data[vcPNIDKey] == vcPNID && configMap.Data[vcPortKey] == vcPort {
		// No update needed.
		return false, nil
	}

	origConfigMap := configMap.DeepCopy()
	configMap.Data[vcPNIDKey] = vcPNID
	configMap.Data[vcPortKey] = vcPort

	err = client.Patch(ctx, configMap, ctrlclient.MergeFrom(origConfigMap))
	if err != nil {
		log.Error(err, "Failed to update provider ConfigMap", "configMapName", configMap.Name)
		return false, err
	}

	return true, nil
}
