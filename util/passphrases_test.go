// Copyright © 2019 Weald Technology Trading
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util_test

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/wealdtech/ethdo/util"
)

func TestGetStorePassphrase(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
	}{
		{
			name:       "Good",
			passphrase: "pass",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("store-passphrase", test.passphrase)
			require.Equal(t, test.passphrase, util.GetStorePassphrase())
		})
	}
}

func TestGetWalletPassphrase(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
	}{
		{
			name:       "Good",
			passphrase: "pass",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("wallet-passphrase", test.passphrase)
			require.Equal(t, test.passphrase, util.GetWalletPassphrase())
		})
	}
}

func TestGetPassphrases(t *testing.T) {
	tests := []struct {
		name        string
		passphrases []string
	}{
		{
			name:        "Single",
			passphrases: []string{"pass"},
		},
		{
			name:        "Multi",
			passphrases: []string{"pass1", "pass2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			if len(test.passphrases) == 1 {
				viper.Set("passphrase", test.passphrases[0])
			} else {
				viper.Set("passphrase", test.passphrases)
			}
			require.Equal(t, test.passphrases, util.GetPassphrases())
		})
	}
}

func TestGetPassphrase(t *testing.T) {
	tests := []struct {
		name        string
		passphrases interface{}
		err         string
	}{
		{
			name: "None",
			err:  "passphrase is required",
		},
		{
			name:        "Single",
			passphrases: "pass",
		},
		{
			name:        "Multi",
			passphrases: []string{"pass1", "pass2"},
			err:         "multiple passphrases supplied",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("passphrase", test.passphrases)
			res, err := util.GetPassphrase()
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.passphrases, res)
			}
		})
	}
}

func TestGetOptionalPassphrase(t *testing.T) {
	tests := []struct {
		name        string
		passphrases interface{}
		err         string
	}{
		{
			name:        "None",
			passphrases: "",
		},
		{
			name:        "Single",
			passphrases: "pass",
		},
		{
			name:        "Multi",
			passphrases: []string{"pass1", "pass2"},
			err:         "multiple passphrases supplied",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("passphrase", test.passphrases)
			res, err := util.GetOptionalPassphrase()
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.passphrases, res)
			}
		})
	}
}
