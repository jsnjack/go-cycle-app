package cmd

// StravaAuthURL is the URL of oauth endpoint
const StravaAuthURL = "https://www.strava.com/oauth/token"

// StravaUploadURL is the URL of Strava upload endpoint
const StravaUploadURL = "https://www.strava.com/api/v3/uploads"

// StravaResponseRefresh represents JSON data that comes from Strava
type StravaResponseRefresh struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int    `json:"expires_at"`
}

// StravaResponseAuth ...
type StravaResponseAuth struct {
	StravaResponseRefresh
	Athlete AthleteData `json:"athlete"`
}

// AthleteData is the `athlete` key in StravaResponse
type AthleteData struct {
	ID int `json:"id"`
}
