// Copyright (c) 2024 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package vmwatcher_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVMWatcherService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VM Watcher Service Test Suite")
}
