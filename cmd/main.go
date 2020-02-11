/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"time"

	"k8s.io/klog"
	"k8s.io/utils/mount"

	gce "sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/pkg/gce-cloud-provider/compute"
	metadataservice "sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/pkg/gce-cloud-provider/metadata"
	driver "sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/pkg/gce-pd-csi-driver"
	mountmanager "sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/pkg/mount-manager"
)

var (
	cloudConfigFilePath  = flag.String("cloud-config", "", "Path to GCE cloud provider config")
	endpoint             = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	runControllerService = flag.Bool("run-controller-service", true, "If set to false then the CSI driver does not activate its controller service (default: true)")
	runNodeService       = flag.Bool("run-node-service", true, "If set to false then the CSI driver does not activate its node service (default: true)")
	vendorVersion        string
)

const (
	driverName = "pd.csi.storage.gke.io"
)

func init() {
	// klog verbosity guide for this package
	// Use V(2) for one time config information
	// Use V(4) for general debug information logging
	// Use V(5) for GCE Cloud Provider Call informational logging
	// Use V(6) for extra repeated/polling information
	klog.InitFlags(flag.CommandLine)
	flag.Set("logtostderr", "true")
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	handle()
	os.Exit(0)
}

func handle() {
	var err error

	if vendorVersion == "" {
		klog.Fatalf("vendorVersion must be set at compile time")
	}
	klog.V(2).Infof("Driver vendor version %v", vendorVersion)

	gceDriver := driver.GetGCEDriver()

	//Initialize GCE Driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Initialize requirements for the controller service
	var (
		cloudProvider gce.GCECompute
	)
	if *runControllerService {
		cloudProvider, err = gce.CreateCloudProvider(ctx, vendorVersion, *cloudConfigFilePath)
		if err != nil {
			klog.Fatalf("Failed to get cloud provider: %v", err)
		}
	} else if *cloudConfigFilePath != "" {
		klog.Warningf("controller service is disabled but cloud config given - it has no effect")
	}

	//Initialize requirements for the node service
	var (
		mounter     *mount.SafeFormatAndMount
		deviceUtils mountmanager.DeviceUtils
		statter     mountmanager.Statter
		meta        metadataservice.MetadataService
	)
	if *runNodeService {
		mounter = mountmanager.NewSafeMounter()
		deviceUtils = mountmanager.NewDeviceUtils()
		statter = mountmanager.NewStatter()
		meta, err = metadataservice.NewMetadataService()
		if err != nil {
			klog.Fatalf("Failed to set up metadata service: %v", err)
		}
	}

	err = gceDriver.SetupGCEDriver(cloudProvider, mounter, deviceUtils, meta, statter, driverName, vendorVersion)
	if err != nil {
		klog.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	gceDriver.Run(*endpoint)
}
