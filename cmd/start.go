/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"crypto/tls"
	"net/http"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/acme/autocert"

	bolt "go.etcd.io/bbolt"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the webserver",
	RunE: func(cmd *cobra.Command, args []string) error {
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

func init() {
	rootCmd.AddCommand(startCmd)
}
