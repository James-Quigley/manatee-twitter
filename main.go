package main

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/joho/godotenv"

	"github.com/ChimeraCoder/anaconda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var BUCKET string = "quaki-manatee-pics"
var TABLE string = "quaki-manatee-pics"

func getImageList(svc *s3.S3) (*s3.ListObjectsOutput, error) {
	log.Println("Getting image list...")
	output, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: &BUCKET,
		Prefix: aws.String("unused"),
	})
	log.Println("Got images")
	return output, err
}

func moveImageToUsed(svc *s3.S3, image *s3.Object) {
	newKey := strings.Replace(*image.Key, "unused", "used", 1)
	copySource := BUCKET + "/" + *image.Key
	_, err := svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     &BUCKET,
		Key:        &newKey,
		CopySource: &copySource,
	})
	if err != nil {
		log.Fatalf("Failed to copy object, %v", err)
	}

	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &BUCKET,
		Key:    image.Key,
	})

	if err != nil {
		log.Fatalf("Failed to delete object, %v", err)
	}
}

func moveAllToUnused(svc *s3.S3) {
	log.Println("Moving all images back to unused")
	output, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: &BUCKET,
		Prefix: aws.String("used"),
	})
	if err != nil {
		log.Fatalf("Error getting images: %v", err)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(output.Contents))
	for _, object := range output.Contents {
		go func(o *s3.Object) {
			newKey := strings.Replace(*o.Key, "used", "unused", 1)
			copySource := BUCKET + "/" + *o.Key
			_, err := svc.CopyObject(&s3.CopyObjectInput{
				Bucket:     &BUCKET,
				Key:        &newKey,
				CopySource: &copySource,
			})
			if err != nil {
				log.Fatalf("Failed to copy object, %v", err)
			}

			_, err = svc.DeleteObject(&s3.DeleteObjectInput{
				Bucket: &BUCKET,
				Key:    o.Key,
			})
			if err != nil {
				log.Fatalf("Failed to delete object, %v", err)
			}
			log.Printf("Deleted used object: %s", *o.Key)
			wg.Done()
		}(object)
	}
	wg.Wait()
}

func HandleRequest() error {
	err := godotenv.Load()

	twitterAccessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	twitterAccessTokenSecret := os.Getenv("TWITTER_ACCESS_TOKEN_SECRET")
	twitterConsumerKey := os.Getenv("TWITTER_CONSUMER_KEY")
	twitterConsumerSecret := os.Getenv("TWITTER_CONSUMER_SECRET")

	if twitterAccessToken == "" || twitterAccessTokenSecret == "" || twitterConsumerKey == "" || twitterConsumerSecret == "" {
		log.Fatalln("Missing required Twitter environment variables")
	}

	api := anaconda.NewTwitterApiWithCredentials(
		twitterAccessToken,
		twitterAccessTokenSecret,
		twitterConsumerKey,
		twitterConsumerSecret)

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(endpoints.UsEast1RegionID),
	}))
	svc := s3.New(sess)

	objects, err := getImageList(svc)
	if err != nil {
		log.Fatalf("Error getting images: %v", err)
	}
	image := objects.Contents[0]
	for {
		idx := rand.Intn(len(objects.Contents))
		image = objects.Contents[idx]
		log.Printf("Considering image: %s", *image.Key)
		if strings.HasSuffix(*image.Key, ".jpg") {
			break
		}
	}
	log.Printf("Using image: %s", *image.Key)

	downloader := s3manager.NewDownloader(sess)

	fileKey := "/tmp/" + strings.Replace(*image.Key, "unused/", "", 1)
	file, err := os.Create(fileKey)
	if err != nil {
		log.Fatalf("Failed to create tmp file location: %v", err)
	}
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(BUCKET),
			Key:    aws.String(*image.Key),
		})

	if err != nil {
		log.Fatalf("Unable to download file: %v", err)
	}

	data, err := ioutil.ReadFile(file.Name())
	if err != nil {
		log.Fatalf("Unable to read file: %v", err)
	}
	base64Str := base64.StdEncoding.EncodeToString(data)

	media, err := api.UploadMedia(base64Str)
	if err != nil {
		log.Fatalf("Failed to upload image: %v", err)
	}

	v := url.Values{}
	v.Set("media_ids", media.MediaIDString)
	_, err = api.PostTweet("", v)
	if err != nil {
		log.Fatalf("Failed to post tweet: %v", err)
	}

	if len(objects.Contents) == 2 {
		moveAllToUnused(svc)
	} else {
		moveImageToUsed(svc, image)
	}
	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
