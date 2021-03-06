/*
   Copyright 2019 Splunk Inc.

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

package commands

import (
	"regexp"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	first := true
	var captured remote.SyncOptions
	s.client.syncFunc = func(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
		if first {
			first = false
			captured = opts
		}
		switch {
		case obj.GetName() == "svc2-cm":
			return &remote.SyncResult{Type: remote.SyncUpdated, Details: "data updated"}, nil
		case obj.GetName() == "svc2-secret":
			return &remote.SyncResult{Type: remote.SyncCreated, Details: "some yaml"}, nil
		default:
			return &remote.SyncResult{Type: remote.SyncObjectsIdentical, Details: "sync skipped"}, nil
		}
	}
	err := s.executeCommand("apply", "dev", "--gc=false")
	require.Nil(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues(remote.SyncOptions{}, captured)
	a.True(stats["same"].(float64) > 0)
	a.EqualValues(7, stats["same"])
	a.EqualValues([]interface{}{"Secret:bar-system:svc2-secret"}, stats["created"])
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm"}, stats["updated"])
	s.assertErrorLineMatch(regexp.MustCompile(`sync ConfigMap:bar-system:svc2-cm`))
}

func TestApplyFlags(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	first := true
	var captured remote.SyncOptions
	s.client.syncFunc = func(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
		if first {
			first = false
			captured = opts
		}
		switch {
		case obj.GetName() == "svc2-cm":
			return &remote.SyncResult{Type: remote.SyncUpdated, Details: "data updated"}, nil
		case obj.GetName() == "svc2-secret":
			return &remote.SyncResult{Type: remote.SyncSkip, Details: "create skipped"}, nil
		default:
			return &remote.SyncResult{Type: remote.SyncObjectsIdentical, Details: "sync skipped"}, nil
		}
	}
	err := s.executeCommand("apply", "dev", "-S", "-n", "--skip-create", "--gc=false")
	require.Nil(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues(remote.SyncOptions{DryRun: true, ShowSecrets: true, DisableCreate: true}, captured)
	a.EqualValues(nil, stats["created"])
	a.EqualValues([]interface{}{"Secret:bar-system:svc2-secret"}, stats["skipped"])
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm"}, stats["updated"])
	s.assertErrorLineMatch(regexp.MustCompile(`\[dry-run\] sync ConfigMap:bar-system:svc2-cm`))
	s.assertErrorLineMatch(regexp.MustCompile(`\*\* dry-run mode, nothing was actually changed \*\*`))
}

func TestApplyNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "no env",
			args: []string{"apply"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"apply", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"apply", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
				a.Equal("invalid environment \"foo\"", err.Error())
			},
		},
		{
			name: "baseline env",
			args: []string{"apply", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("cannot apply baseline environment, use a real environment", err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"apply", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"apply", "dev", "-k", "namespace", "-K", "secret"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`cannot include as well as exclude kinds, specify one or the other`, err.Error())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newScaffold(t)
			defer s.reset()
			err := s.executeCommand(test.args...)
			require.NotNil(t, err)
			test.asserter(s, err)
		})
	}

}
