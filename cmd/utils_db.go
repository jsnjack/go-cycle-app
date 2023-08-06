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

// DB structure:
// There are 2 buckets:
// 1. AccountBucket - contains all information about Strava athlete: access token and athlet's goal
// 2. AccountAliasBucket - maps athlete's account ID in go-cycle app to his athlete ID in Strava

var AccountBucket = []byte("account")
var AccountAliasBucket = []byte("alias")

// RefreshAccessToken refresh access token
func RefreshAccessToken(athleteID int) (string, error) {
	var refreshToken string
	err := DB.View(func(tx *bolt.Tx) error {
		authBucket := tx.Bucket(AccountBucket)

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
		authBucket := tx.Bucket(AccountBucket)

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

func CreateAccountAlias(appAccountID string, athleteID int) error {
	err := DB.Update(func(tx *bolt.Tx) error {
		aliasBucket := tx.Bucket(AccountAliasBucket)
		err := aliasBucket.Put([]byte("appAccountID"), []byte(fmt.Sprintf("%d", athleteID)))
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func GetAthleteIDFromAccountID(appAccountID string) (int, error) {
	var athleteID int
	err := DB.View(func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket(AccountAliasBucket)
		result := bucket.Get([]byte(appAccountID))
		if result == nil {
			return fmt.Errorf("account %s doesn't exist", appAccountID)
		}
		athleteIDStr := string(result)
		athleteID, err = strconv.Atoi(athleteIDStr)
		return err
	})

	return athleteID, err
}

func SetGoal(athleteID int, goal float64) error {
	err := DB.View(func(tx *bolt.Tx) error {
		authBucket := tx.Bucket(AccountBucket)

		bucket := authBucket.Bucket([]byte(fmt.Sprintf("%d", athleteID)))
		if bucket == nil {
			return fmt.Errorf("user with athleteID %d doesn't exist", athleteID)
		}

		err := bucket.Put([]byte("goal"), []byte(fmt.Sprintf("%f", goal*1000)))
		return err
	})
	return err
}

func GetGoal(athleteID int) (float64, error) {
	var goal float64
	err := DB.View(func(tx *bolt.Tx) error {
		var err error
		authBucket := tx.Bucket(AccountBucket)
		bucket := authBucket.Bucket([]byte(fmt.Sprintf("%d", athleteID)))
		if bucket == nil {
			return fmt.Errorf("user with athleteID %d doesn't exist", athleteID)
		}
		goalBytes := bucket.Get([]byte("goal"))
		if goalBytes == nil {
			return fmt.Errorf("goal for athleteID %d is not found", athleteID)
		}
		goalStr := string(goalBytes)
		goal, err = strconv.ParseFloat(goalStr, 64)
		return err
	})
	return goal, err
}
