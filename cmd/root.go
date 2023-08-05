/*
Copyright © 2023 YAUHEN SHULITSKI
*/
package cmd

import (
	"crypto/tls"
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/acme/autocert"
)

var rootPort string
var rootDomain string
var rootAppID string
var rootAppSecret string
var rootDBFilename string

// DB is the Bolt db
var DB *bolt.DB

// Version is the version of the application calculated with monova
var Version string

// Logger is the main logger
var Logger *log.Logger

//go:embed templates/*
var TemplatesStorage embed.FS

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gocycleapp",
	Short: "go-cycle Strava application",
	RunE: func(cmd *cobra.Command, args []string) error {
		Logger.Printf("Starting the application on port %s, domain %s and db %s ...\n", rootPort, rootDomain, rootDBFilename)
		var err error
		DB, err = bolt.Open(rootDBFilename, 0644, nil)
		if err != nil {
			Logger.Fatal(err)
		}
		defer DB.Close()

		err = DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists(AuthBucket)
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			Logger.Fatal(err)
		}

		http.Handle("/", logMi(sslChallenge))
		http.Handle("/connect", logMi(connectRequest))
		http.Handle("/register", logMi(register))
		http.Handle("/upload", logMi(upload))
		http.Handle("/register/success", logMi(registerSuccess))

		if rootPort == "443" {
			certManager := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      autocert.DirCache("certs"),
				HostPolicy: autocert.HostWhitelist(rootDomain),
			}

			go func() {
				Logger.Println("Listening on port 80...")
				err := http.ListenAndServe(":80", certManager.HTTPHandler(nil))
				if err != nil {
					Logger.Fatal(err)
				}
			}()

			Logger.Println("Listening on port 443...")
			server := &http.Server{
				Addr: ":443",
				TLSConfig: &tls.Config{
					GetCertificate: certManager.GetCertificate,
				},
			}

			err = server.ListenAndServeTLS("", "")
			if err != nil {
				Logger.Fatal(err)
			}
		} else {
			Logger.Printf("Listening on port %s...\n", rootPort)
			err := http.ListenAndServe(":"+rootPort, nil)
			if err != nil {
				Logger.Fatal(err)
			}
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gocycleapp.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringVarP(&rootPort, "port", "p", "8080", "Port to start the server on")
	rootCmd.Flags().StringVarP(&rootDomain, "domain", "d", "localhost", "Webserver domain name. Used when port is 443 for the certificat retrieval")
	rootCmd.Flags().StringVarP(&rootAppID, "id", "i", "", "Strava application ID")
	rootCmd.Flags().StringVarP(&rootAppSecret, "secret", "s", "", "Strava application secret")
	rootCmd.Flags().StringVarP(&rootDBFilename, "filename", "f", "go-cycle-app.db", "DB filename")

	Logger = log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile)
}
