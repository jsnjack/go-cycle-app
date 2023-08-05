package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// AuthBucket is the name of the bucket to store auth data from Strava  in
var AuthBucket = []byte("auth")

// RefreshAccessToken refresh access token
func RefreshAccessToken(athleteID int) (string, error) {
	var refreshToken string
	err := DB.View(func(tx *bolt.Tx) error {
		authBucket := tx.Bucket(AuthBucket)

		bucket := authBucket.Bucket([]byte(fmt.Sprintf("%d", athleteID)))
		if bucket == nil {
			return fmt.Errorf("user with athleteID %d doesn't exist", athleteID)
		}

		data := bucket.Get([]byte("refreshToken"))
		if data == nil {
			return fmt.Errorf("refresh token for athleteID %d is not found", athleteID)
		}
		refreshToken = string(data)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Request token
	form := url.Values{}
	form.Add("client_id", rootAppID)
	form.Add("client_secret", rootAppSecret)
	form.Add("grant_type", "refresh_token")
	form.Add("refresh_token", refreshToken)
	formData := form.Encode()
	req, _ := http.NewRequest("POST", StravaAuthURL, strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err := fmt.Errorf("unexpected status code from Strava API: %d", resp.StatusCode)
		return "", err
	}

	// Parse response
	bodyByte, _ := io.ReadAll(resp.Body)
	var stravaData StravaResponseRefresh
	err = json.Unmarshal(bodyByte, &stravaData)
	if err != nil {
		return "", err
	}

	err = SaveAuthData(athleteID, &stravaData)

	if err != nil {
		return "", err
	}

	return stravaData.AccessToken, nil
}

// SaveAuthData saves data retrieved from Strava to the database
func SaveAuthData(athleteID int, data *StravaResponseRefresh) error {
	err := DB.Update(func(tx *bolt.Tx) error {
		authBucket := tx.Bucket(AuthBucket)

		athleteBucket, err := authBucket.CreateBucketIfNotExists([]byte(fmt.Sprintf("%d", athleteID)))
		if err != nil {
			return err
		}

		err = athleteBucket.Put([]byte("accessToken"), []byte(data.AccessToken))
		if err != nil {
			return err
		}

		err = athleteBucket.Put([]byte("refreshToken"), []byte(data.RefreshToken))
		if err != nil {
			return err
		}

		err = athleteBucket.Put([]byte("expiresAt"), []byte(strconv.Itoa(data.ExpiresAt)))
		if err != nil {
			return err
		}
		return nil
	})
	return err
}
