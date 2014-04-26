package main

import (
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/storage/v1beta2"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const (
	// Some constants for our project
	bucketName   = "{{BUCKET_NAME}}"
	projectID    = "{{PROJECT_ID}}"
	clientId     = "{{CLIENT_ID}}"
	clientSecret = "{{CLIENT_SECRETS}}"

	fileName   = "/usr/share/dict/words"
	objectName = "english-dictionary"
)

var (
	cacheFile = flag.String("cache", "cache.json", "Token cache file")
	code      = flag.String("code", "", "Authorization Code")
)

// getAuthenticatedClient does the heavy lifting to get an OAuth2-enabled http.Client.
func getAuthenticatedClient() *http.Client {
	config := &oauth.Config{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Scope:        storage.DevstorageFull_controlScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   oauth.CacheFile(*cacheFile),
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	transport := &oauth.Transport{
		Config:    config,
		Transport: http.DefaultTransport,
	}

	token, err := config.TokenCache.Token()
	if err != nil {
		if *code == "" {
			url := config.AuthCodeURL("")
			log.Fatalf("Visit URL to get a code then run again with -code=YOUR_CODE\n%s", url)
		}

		// Exchange auth code for access token
		token, err = transport.Exchange(*code)
		if err != nil {
			log.Fatal("Exchange: ", err)
		}
		log.Printf("Token is cached in %v\n", config.TokenCache)
	}
	transport.Token = token

	return transport.Client()
}

func main() {
	flag.Parse()

	httpClient := getAuthenticatedClient()
	service, err := storage.New(httpClient)

	// Check to see if the specified bucket exists, and create it if necessary
	if _, err := service.Buckets.Get(bucketName).Do(); err == nil {
		log.Printf("Bucket %s already exists - skipping buckets.insert call.\n", bucketName)
	} else {
		if res, err := service.Buckets.Insert(projectID, &storage.Bucket{Name: bucketName}).Do(); err != nil {
			log.Fatalf("Failed creating bucket %s: %v", bucketName, err)
		} else {
			log.Printf("Created bucket %v at location %v\n\n", res.Name, res.SelfLink)
		}
	}

	// Open the file specified above, and upload its contents to our bucket with the specified object name
	object := &storage.Object{Name: objectName}
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Error opening %q: %v", fileName, err)
	}
	if res, err := service.Objects.Insert(bucketName, object).Media(file).Do(); err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	} else {
		log.Printf("Created object %v at location %v\n", res.Name, res.SelfLink)
	}

	// Fetch the media link for the object
	res, err := service.Objects.Get(bucketName, objectName).Do()
	if err != nil {
		log.Fatalf("Failed to get %s/%s: %s.", bucketName, objectName, err)
	}
	// Using the media link, grab the object itself
	log.Printf("Downloading media from %s\n", res.MediaLink)
	objectResponse, err := httpClient.Get(res.MediaLink)
	if err != nil {
		log.Fatalf("Unable to fetch object %s.", objectName)
	}
	// Pull the data out of the response body
	objectContents, err := ioutil.ReadAll(objectResponse.Body)
	objectResponse.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	// Finally, print the object's contents
	log.Printf("Object contents:\n%s", objectContents)
}
