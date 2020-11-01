// Copyright © 2019, 2020 Weald Technology Trading
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

package depositdata

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/wealdtech/ethdo/core"
	"github.com/wealdtech/ethdo/grpc"
	e2types "github.com/wealdtech/go-eth2-types/v2"
	util "github.com/wealdtech/go-eth2-util"
	e2wtypes "github.com/wealdtech/go-eth2-wallet-types/v2"
	string2eth "github.com/wealdtech/go-string2eth"
)

type dataIn struct {
	format                string
	withdrawalCredentials []byte
	amount                uint64
	validatorAccounts     []e2wtypes.Account
	forkVersion           []byte
	domain                []byte
}

func input() (*dataIn, error) {
	var err error
	data := &dataIn{}

	if viper.GetString("validatoraccount") == "" {
		return nil, errors.New("validator account is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
	defer cancel()
	_, data.validatorAccounts, err = core.WalletAndAccountsFromPath(ctx, viper.GetString("validatoraccount"))
	if err != nil {
		return nil, errors.New("failed to obtain validator account")
	}
	if len(data.validatorAccounts) == 0 {
		return nil, errors.New("unknown validator account")
	}

	switch {
	case viper.GetBool("launchpad"):
		data.format = "launchpad"
	case viper.GetBool("raw"):
		data.format = "raw"
	default:
		data.format = "json"
	}

	switch {
	case viper.GetString("withdrawalaccount") != "":
		ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
		defer cancel()
		_, withdrawalAccount, err := core.WalletAndAccountFromPath(ctx, viper.GetString("withdrawalaccount"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain withdrawal account")
		}
		pubKey, err := core.BestPublicKey(withdrawalAccount)
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain public key for withdrawal account")
		}
		data.withdrawalCredentials = util.SHA256(pubKey.Marshal())
	case viper.GetString("withdrawalpubkey") != "":
		withdrawalPubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(viper.GetString("withdrawalpubkey"), "0x"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode withdrawal public key")
		}
		if len(withdrawalPubKeyBytes) != 48 {
			return nil, errors.New("withdrawal public key must be exactly 48 bytes in length")
		}
		withdrawalPubKey, err := e2types.BLSPublicKeyFromBytes(withdrawalPubKeyBytes)
		if err != nil {
			return nil, errors.Wrap(err, "withdrawal public key is not valid")
		}
		data.withdrawalCredentials = util.SHA256(withdrawalPubKey.Marshal())
	default:
		return nil, errors.New("withdrawalaccount or withdrawal public key is required")
	}
	// This is hard-coded, to allow deposit data to be generated without a connection to the beacon node.
	data.withdrawalCredentials[0] = byte(0) // BLS_WITHDRAWAL_PREFIX

	if viper.GetString("depositvalue") == "" {
		return nil, errors.New("deposit value is required")
	}
	data.amount, err = string2eth.StringToGWei(viper.GetString("depositvalue"))
	if err != nil {
		return nil, errors.Wrap(err, "deposit value is invalid")
	}
	// This is hard-coded, to allow deposit data to be generated without a connection to the beacon node.
	if data.amount < 1000000000 { // MIN_DEPOSIT_AMOUNT
		return nil, errors.New("deposit value must be at least 1 Ether")
	}

	if viper.GetString("forkversion") != "" {
		data.forkVersion, err = hex.DecodeString(strings.TrimPrefix(viper.GetString("forkversion"), "0x"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode fork version")
		}
		if len(data.forkVersion) != 4 {
			return nil, errors.New("fork version must be exactly 4 bytes in length")
		}
	} else {
		conn, err := grpc.Connect()
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to beacon node")
		}
		config, err := grpc.FetchChainConfig(conn)
		if err != nil {
			return nil, errors.Wrap(err, "could not connect to beacon node; supply a connection with --connection or provide a fork version with --forkversion to generate deposit data")
		}
		genesisForkVersion, exists := config["GenesisForkVersion"]
		if !exists {
			return nil, errors.New("failed to obtain genesis fork version")
		}
		data.forkVersion = genesisForkVersion.([]byte)
	}
	data.domain = e2types.Domain(e2types.DomainDeposit, data.forkVersion, e2types.ZeroGenesisValidatorsRoot)

	return data, nil
}
