package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/storage/v1beta2"
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

// authClient does the heavy lifting to get an OAuth2-enabled http.Client.
func authClient(cacheFile, code string) (*http.Client, error) {
	config := &oauth.Config{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Scope:        storage.DevstorageFull_controlScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   oauth.CacheFile(cacheFile),
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	transport := &oauth.Transport{
		Config:    config,
		Transport: http.DefaultTransport,
	}

	token, err := config.TokenCache.Token()
	if err != nil {
		if code == "" {
			url := config.AuthCodeURL("")
			return nil, fmt.Errorf("Visit URL to get a code then run again with -code=YOUR_CODE\n%s", url)
		}

		// Exchange auth code for access token
		token, err = transport.Exchange(code)
		if err != nil {
			return nil, err
		}
		log.Printf("Token is cached in %v\n", config.TokenCache)
	}
	transport.Token = token

	return transport.Client(), nil
}

func main() {
	// Sets up two command line flags
	cacheFile := flag.String("cache", "cache.json", "Token cache file")
	code := flag.String("code", "", "Authorization Code")
	flag.Parse()

	// Creates an oauth-enabled HTTP client
	httpClient, err := authClient(*cacheFile, *code)
	if err != nil {
		log.Fatal(err)
	}
	service, err := storage.New(httpClient)

	// Check to see if the specified bucket exists, and create it if necessary
	if _, err := service.Buckets.Get(bucketName).Do(); err == nil {
		log.Printf("Bucket %s already exists - skipping buckets.insert call.\n", bucketName)
	} else {
		res, err := service.Buckets.Insert(projectID, &storage.Bucket{Name: bucketName}).Do()
		if err != nil {
			log.Fatalf("Failed creating bucket %s: %v", bucketName, err)
		}
		log.Printf("Created bucket %v at location %v\n\n", res.Name, res.SelfLink)
	}

	// Open the file specified above, and upload its contents to our bucket with the specified object name
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Error opening %q: %v", fileName, err)
	}
	defer file.Close()
	obj, err := service.Objects.Insert(bucketName, &storage.Object{Name: objectName}).Media(file).Do()
	if err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	}
	log.Printf("Created object %v at location %v\n", obj.Name, obj.SelfLink)

	// Fetch the media link for the object
	obj, err = service.Objects.Get(bucketName, objectName).Do()
	if err != nil {
		log.Fatalf("Failed to get %s/%s: %s.", bucketName, objectName, err)
	}
	// Using the media link, grab the object itself
	log.Printf("Downloading media from %s\n", obj.MediaLink)
	media, err := httpClient.Get(obj.MediaLink)
	if err != nil {
		log.Fatalf("Unable to fetch object %s.", objectName)
	}
	defer media.Body.Close()
	// Pull the data out of the response body
	content, err := ioutil.ReadAll(media.Body)
	if err != nil {
		log.Fatalf("Couldn't read response body when fetching object: %s", err)
	}
	// Finally, print the object's contents
	log.Printf("Object contents:\n%s", content)
}
