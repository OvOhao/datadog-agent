// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package fx provides the Fx module for Azure App Service extension inventory.
package fx

import (
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/config"
	runnerfx "github.com/DataDog/datadog-agent/comp/metadata/runner/fx"
	serverlessaasimpl "github.com/DataDog/datadog-agent/comp/metadata/serverlessaas/impl"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

// Module enables only AAS inventory and its runner within this Fx scope. The
// extension's global metadata setting remains disabled for host collectors.
func Module() fxutil.Module {
	return fxutil.Component(
		fx.Decorate(func(conf config.Component) config.Component { return inventoryConfig{conf} }),
		runnerfx.Module(),
		fxutil.ProvideComponentConstructor(serverlessaasimpl.NewComponent),
	)
}

type inventoryConfig struct{ config.Component }

func (c inventoryConfig) GetBool(key string) bool {
	if key == "enable_metadata_collection" {
		return true
	}
	return c.Component.GetBool(key)
}
