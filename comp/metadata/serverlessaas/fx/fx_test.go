// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package fx

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/config"
	hostnameinterface "github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface/def"
	log "github.com/DataDog/datadog-agent/comp/core/log/def"
	logmock "github.com/DataDog/datadog-agent/comp/core/log/mock"
	"github.com/DataDog/datadog-agent/comp/metadata/inventorypayload"
	runner "github.com/DataDog/datadog-agent/comp/metadata/runner/def"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

type recordingSerializer struct {
	serializer.MetricSerializer
	payloads []*inventorypayload.Payload
}

func (s *recordingSerializer) SendMetadata(payload marshaler.JSONMarshaler) error {
	s.payloads = append(s.payloads, payload.(*inventorypayload.Payload))
	return nil
}

type fakeHostname struct{ hostnameinterface.Component }

func (fakeHostname) Get(context.Context) (string, error) { return "", nil }
func (fakeHostname) GetWithProvider(context.Context) (hostnameinterface.Data, error) {
	return hostnameinterface.Data{}, nil
}

func TestScopedRunner(t *testing.T) {
	for _, tc := range []struct {
		name       string
		flag       string
		inventory  bool
		identity   bool
		wantReport bool
	}{
		{"extension defaults", "1", true, true, true},
		{"flag off", "0", true, true, false},
		{"inventory disabled", "1", false, true, false},
		{"missing identity", "1", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DD_SERVERLESS_AAS_EXTENSION_INVENTORY_ENABLED", tc.flag)
			t.Setenv("WEBSITE_SITE_NAME", "")
			if tc.identity {
				t.Setenv("WEBSITE_SITE_NAME", "test-app")
			}
			t.Setenv("WEBSITE_OWNER_NAME", "sub-123+East US")
			t.Setenv("WEBSITE_RESOURCE_GROUP", "test-rg")
			synctest.Test(t, func(t *testing.T) {
				conf := config.NewMockWithOverrides(t, map[string]any{
					"enable_metadata_collection":        false,
					"inventories_enabled":               tc.inventory,
					"inventories_configuration_enabled": false,
					"inventories_first_run_delay":       3600,
					"inventories_min_interval":          60,
					"inventories_max_interval":          600,
				})
				s := &recordingSerializer{}
				var rootConfig config.Component
				app := fx.New(
					fx.NopLogger,
					fxutil.FxLifecycleAdapter(),
					fx.Provide(func() config.Component { return conf }),
					fx.Provide(func() log.Component { return logmock.New(t) }),
					fx.Provide(func() hostnameinterface.Component { return fakeHostname{} }),
					fx.Provide(func() serializer.MetricSerializer { return s }),
					Module(),
					fx.Populate(&rootConfig),
					fx.Invoke(func(_ runner.Component) {}),
					fx.Invoke(func(in struct {
						fx.In
						Providers []runner.MetadataProvider `group:"metadata_provider"`
					}) {
						expected := 0
						if tc.flag == "1" && tc.inventory {
							expected = 1
						}
						require.Len(t, runner.GetAndFilterProviders(in.Providers), expected)
					}),
				)
				require.NoError(t, app.Err())
				require.Empty(t, s.payloads, "construction must not submit")
				require.NoError(t, app.Start(t.Context()))
				synctest.Wait()
				require.False(t, rootConfig.GetBool("enable_metadata_collection"), "the override must not escape the AAS scope")
				require.Equal(t, tc.inventory, rootConfig.GetBool("inventories_enabled"))
				if tc.wantReport {
					require.Len(t, s.payloads, 1)
					require.Equal(t, "serverless-extension", s.payloads[0].Metadata["flavor"])
					require.Equal(t, "/subscriptions/sub-123/resourcegroups/test-rg/providers/microsoft.web/sites/test-app", s.payloads[0].Metadata["resource_id"])
					time.Sleep(10 * time.Minute) // Advance virtual time to the next report.
					synctest.Wait()
					require.Len(t, s.payloads, 2)
					require.Equal(t, "periodic", s.payloads[1].Metadata["report_reason"])
				} else {
					require.Empty(t, s.payloads)
				}
				require.NoError(t, app.Stop(t.Context()))
				count := len(s.payloads)
				time.Sleep(20 * time.Minute) // Virtual time: a stopped runner must not report again.
				synctest.Wait()
				require.Len(t, s.payloads, count)
			})
		})
	}
}
