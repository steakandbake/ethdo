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

package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	e2types "github.com/wealdtech/go-eth2-types/v2"
	e2wallet "github.com/wealdtech/go-eth2-wallet"
	dirk "github.com/wealdtech/go-eth2-wallet-dirk"
	filesystem "github.com/wealdtech/go-eth2-wallet-store-filesystem"
	s3 "github.com/wealdtech/go-eth2-wallet-store-s3"
	e2wtypes "github.com/wealdtech/go-eth2-wallet-types/v2"
	"google.golang.org/grpc"
)

var cfgFile string
var quiet bool
var verbose bool
var debug bool

// Root variables, present for all commands.
var rootStore string

// Store for wallet actions.
var store e2wtypes.Store

// Remote connection.
var remote bool

// Prysm connection.
var eth2GRPCConn *grpc.ClientConn

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:              "ethdo",
	Short:            "Ethereum 2 CLI",
	Long:             `Manage common Ethereum 2 tasks from the command line.`,
	PersistentPreRun: persistentPreRun,
}

func persistentPreRun(cmd *cobra.Command, args []string) {
	if cmd.Name() == "help" {
		// User just wants help
		return
	}

	if cmd.Name() == "version" {
		// User just wants the version
		return
	}

	// We bind viper here so that we bind to the correct command.
	quiet = viper.GetBool("quiet")
	verbose = viper.GetBool("verbose")
	debug = viper.GetBool("debug")
	rootStore = viper.GetString("store")
	// Command-specific bindings.
	switch fmt.Sprintf("%s/%s", cmd.Parent().Name(), cmd.Name()) {
	case "account/create":
		accountCreateBindings()
	case "attester/inclusion":
		attesterInclusionBindings()
	case "exit/verify":
		exitVerifyBindings()
	case "validator/depositdata":
		validatorDepositdataBindings()
	case "wallet/create":
		walletCreateBindings()
	}

	if quiet && verbose {
		fmt.Println("Cannot supply both quiet and verbose flags")
	}
	if quiet && debug {
		fmt.Println("Cannot supply both quiet and debug flags")
	}

	if viper.GetString("remote") == "" {
		// Set up our wallet store
		switch rootStore {
		case "s3":
			assert(viper.GetString("base-dir") == "", "--basedir does not apply for the s3 store")
			var err error
			store, err = s3.New(s3.WithPassphrase([]byte(getStorePassphrase())))
			errCheck(err, "Failed to access Amazon S3 wallet store")
		case "filesystem":
			opts := make([]filesystem.Option, 0)
			if getStorePassphrase() != "" {
				opts = append(opts, filesystem.WithPassphrase([]byte(getStorePassphrase())))
			}
			if viper.GetString("base-dir") != "" {
				opts = append(opts, filesystem.WithLocation(viper.GetString("base-dir")))
			}
			store = filesystem.New(opts...)
		default:
			die(fmt.Sprintf("Unsupported wallet store %s", rootStore))
		}
		err := e2wallet.UseStore(store)
		errCheck(err, "Failed to use defined wallet store")
	} else {
		remote = true
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(_exitFailure)
	}
}

func init() {
	// Initialise our BLS library.
	if err := e2types.InitBLS(); err != nil {
		fmt.Println(err)
		os.Exit(_exitFailure)
	}

	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ethdo.yaml)")
	RootCmd.PersistentFlags().String("log", "", "log activity to the named file (default $HOME/ethdo.log).  Logs are written for every action that generates a transaction")
	if err := viper.BindPFlag("log", RootCmd.PersistentFlags().Lookup("log")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("store", "filesystem", "Store for accounts")
	if err := viper.BindPFlag("store", RootCmd.PersistentFlags().Lookup("store")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("account", "", "Account name (in format \"wallet/account\")")
	if err := viper.BindPFlag("account", RootCmd.PersistentFlags().Lookup("account")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("basedir", "", "Base directory for filesystem wallets")
	if err := viper.BindPFlag("base-dir", RootCmd.PersistentFlags().Lookup("basedir")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("storepassphrase", "", "Passphrase for store (if applicable)")
	if err := viper.BindPFlag("store-passphrase", RootCmd.PersistentFlags().Lookup("storepassphrase")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("walletpassphrase", "", "Passphrase for wallet (if applicable)")
	if err := viper.BindPFlag("wallet-passphrase", RootCmd.PersistentFlags().Lookup("walletpassphrase")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().StringSlice("passphrase", nil, "Passphrase for account (if applicable)")
	if err := viper.BindPFlag("passphrase", RootCmd.PersistentFlags().Lookup("passphrase")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().Bool("quiet", false, "do not generate any output")
	if err := viper.BindPFlag("quiet", RootCmd.PersistentFlags().Lookup("quiet")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().Bool("verbose", false, "generate additional output where appropriate")
	if err := viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().Bool("debug", false, "generate debug output")
	if err := viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("connection", "localhost:4000", "connection to Ethereum 2 node via GRPC")
	if err := viper.BindPFlag("connection", RootCmd.PersistentFlags().Lookup("connection")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().Duration("timeout", 10*time.Second, "the time after which a network request will be considered failed.  Increase this if you are running on an error-prone, high-latency or low-bandwidth connection")
	if err := viper.BindPFlag("timeout", RootCmd.PersistentFlags().Lookup("timeout")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("remote", "", "connection to a remote wallet daemon")
	if err := viper.BindPFlag("remote", RootCmd.PersistentFlags().Lookup("remote")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("client-cert", "", "location of a client certificate file when connecting to the remote wallet daemon")
	if err := viper.BindPFlag("client-cert", RootCmd.PersistentFlags().Lookup("client-cert")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("client-key", "", "location of a client key file when connecting to the remote wallet daemon")
	if err := viper.BindPFlag("client-key", RootCmd.PersistentFlags().Lookup("client-key")); err != nil {
		panic(err)
	}
	RootCmd.PersistentFlags().String("server-ca-cert", "", "location of the server certificate authority certificate when connecting to the remote wallet daemon")
	if err := viper.BindPFlag("server-ca-cert", RootCmd.PersistentFlags().Lookup("server-ca-cert")); err != nil {
		panic(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		errCheck(err, "could not find home directory")

		// Search config in home directory with name ".ethdo" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".ethdo")
	}

	viper.SetEnvPrefix("ETHDO")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		// Don't report lack of config file...
		assert(strings.Contains(err.Error(), "Not Found"), "failed to read configuration")
	}
}

//
// Helpers
//

func outputIf(condition bool, msg string) {
	if condition {
		fmt.Println(msg)
	}
}

// walletFromInput obtains a wallet given the information in the viper variable
// "account", or if not present the viper variable "wallet".
func walletFromInput(ctx context.Context) (e2wtypes.Wallet, error) {
	if viper.GetString("account") != "" {
		return walletFromPath(ctx, viper.GetString("account"))
	}
	return walletFromPath(ctx, viper.GetString("wallet"))
}

// walletFromPath obtains a wallet given a path specification.
func walletFromPath(ctx context.Context, path string) (e2wtypes.Wallet, error) {
	walletName, _, err := e2wallet.WalletAndAccountNames(path)
	if err != nil {
		return nil, err
	}
	if viper.GetString("remote") != "" {
		assert(viper.GetString("client-cert") != "", "remote connections require client-cert")
		assert(viper.GetString("client-key") != "", "remote connections require client-key")
		credentials, err := dirk.ComposeCredentials(ctx, viper.GetString("client-cert"), viper.GetString("client-key"), viper.GetString("server-ca-cert"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to build dirk credentials")
		}

		endpoints, err := remotesToEndpoints([]string{viper.GetString("remote")})
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse remote servers")
		}

		return dirk.OpenWallet(ctx, walletName, credentials, endpoints)
	}
	wallet, err := e2wallet.OpenWallet(walletName)
	if err != nil {
		if strings.Contains(err.Error(), "failed to decrypt wallet") {
			return nil, errors.New("Incorrect store passphrase")
		}
		return nil, err
	}
	return wallet, nil
}

// walletAndAccountFromInput obtains the wallet and account given the information in the viper variable "account".
func walletAndAccountFromInput(ctx context.Context) (e2wtypes.Wallet, e2wtypes.Account, error) {
	return walletAndAccountFromPath(ctx, viper.GetString("account"))
}

// walletAndAccountFromPath obtains the wallet and account given a path specification.
func walletAndAccountFromPath(ctx context.Context, path string) (e2wtypes.Wallet, e2wtypes.Account, error) {
	wallet, err := walletFromPath(ctx, path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "faild to open wallet for account")
	}
	_, accountName, err := e2wallet.WalletAndAccountNames(path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to obtain accout name")
	}
	if accountName == "" {
		return nil, nil, errors.New("no account name")
	}

	if wallet.Type() == "hierarchical deterministic" && strings.HasPrefix(accountName, "m/") {
		assert(getWalletPassphrase() != "", "--walletpassphrase is required for direct path derivations")

		locker, isLocker := wallet.(e2wtypes.WalletLocker)
		if isLocker {
			err = locker.Unlock(ctx, []byte(viper.GetString("wallet-passphrase")))
			if err != nil {
				return nil, nil, errors.New("failed to unlock wallet")
			}
			defer relockAccount(locker)
		}
	}

	accountByNameProvider, isAccountByNameProvider := wallet.(e2wtypes.WalletAccountByNameProvider)
	if !isAccountByNameProvider {
		return nil, nil, errors.New("wallet cannot obtain accounts by name")
	}
	account, err := accountByNameProvider.AccountByName(ctx, accountName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to obtain account")
	}
	return wallet, account, nil
}

// walletAndAccountsFromPath obtains the wallet and matching accounts given a path specification.
func walletAndAccountsFromPath(ctx context.Context, path string) (e2wtypes.Wallet, []e2wtypes.Account, error) {
	wallet, err := walletFromPath(ctx, path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "faild to open wallet for account")
	}

	_, accountSpec, err := e2wallet.WalletAndAccountNames(path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to obtain account specification")
	}
	if accountSpec == "" {
		accountSpec = "^.*$"
	} else {
		accountSpec = fmt.Sprintf("^%s$", accountSpec)
	}
	re := regexp.MustCompile(accountSpec)

	accounts := make([]e2wtypes.Account, 0)
	for account := range wallet.Accounts(ctx) {
		if re.Match([]byte(account.Name())) {
			accounts = append(accounts, account)
		}
	}

	// Tidy up accounts by name.
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Name() < accounts[j].Name()
	})

	return wallet, accounts, nil
}

// connect connects to an Ethereum 2 endpoint.
func connect() error {
	if eth2GRPCConn != nil {
		// Already connected.
		return nil
	}

	connection := ""
	if viper.GetString("connection") != "" {
		connection = viper.GetString("connection")
	}

	if connection == "" {
		return errors.New("no connection")
	}
	outputIf(debug, fmt.Sprintf("Connecting to %s", connection))

	opts := []grpc.DialOption{grpc.WithInsecure()}

	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
	defer cancel()
	var err error
	eth2GRPCConn, err = grpc.DialContext(ctx, connection, opts...)
	return err
}

// bestPublicKey returns the best public key for operations.
// It prefers the composite public key if present, otherwise the public key.
func bestPublicKey(account e2wtypes.Account) (e2types.PublicKey, error) {
	var pubKey e2types.PublicKey
	publicKeyProvider, isCompositePublicKeyProvider := account.(e2wtypes.AccountCompositePublicKeyProvider)
	if isCompositePublicKeyProvider {
		pubKey = publicKeyProvider.CompositePublicKey()
	} else {
		publicKeyProvider, isPublicKeyProvider := account.(e2wtypes.AccountPublicKeyProvider)
		if isPublicKeyProvider {
			pubKey = publicKeyProvider.PublicKey()
		} else {
			return nil, errors.New("account does not provide a public key")
		}
	}
	return pubKey, nil
}

// remotesToEndpoints generates endpoints from remote addresses.
func remotesToEndpoints(remotes []string) ([]*dirk.Endpoint, error) {
	endpoints := make([]*dirk.Endpoint, 0)
	for _, remote := range remotes {
		parts := strings.Split(remote, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid remote %q", remote)
		}
		port, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("invalid port in remote %q", remote))
		}
		endpoints = append(endpoints, dirk.NewEndpoint(parts[0], uint32(port)))
	}
	return endpoints, nil
}

// relockAccount locks an account; generally called as a defer after an account is unlocked.
func relockAccount(locker e2wtypes.AccountLocker) {
	errCheck(locker.Lock(context.Background()), "failed to re-lock account")
}
