package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"bytes"
	"math"
)

// Document : Object for each page with an array for each asset
type Document struct {
	Location    string
	Urls        []string
	Stylesheets []string
	Scripts     []string
	Images      []string
}


//Sleeps for amount of time that would meet the crawl delay enforcing politeness
func delaySecond(delay float64) {
	fmt.Println("Delaying for:", time.Duration(math.Abs(delay)*1000)*time.Millisecond)
	time.Sleep(time.Duration(math.Abs(delay)*1000) * time.Millisecond)
}

func initialiseRobots(newURL string) {
	defer timeTrack(time.Now(), "Initialise Robots")
	u, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	needRefresh := false
	constructURL := u.Scheme + "://" + u.Host + "/robots.txt"

	// check whether the robots.txt has already been downloaded, if not download it
	if _, err := os.Stat(u.Host); os.IsNotExist(err) {
		downloadAndCreateRobots(u, constructURL)
	}

	// fetch the info regarding robot.txt ie modified time
	// check whether the robots.txt needs a refresh depending on if the time since last download is greater than 1 hour
	info, err := os.Stat(u.Host)
	currentTime := time.Now()
	robotsModified := info.ModTime()
	elapsed := currentTime.Sub(robotsModified).Hours()

	if elapsed > 1 {
		needRefresh = true
	}

	if needRefresh {
		downloadAndCreateRobots(u, constructURL)
	}
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

func checkRobots(newURL string, disallowedPages []string) bool {
	// This function checks for politeness regarding user agents, allow/deny and crawler rate
	URL, err := url.Parse(newURL)

	if err != nil {
		fmt.Print(err)
	}

	for _, disallowPath := range disallowedPages {
		if disallowPath == URL.Path {
			return false
		}
	}
	return true

}

//reads the robot.txt then processes it so that a URL can be checked against it quickly
func processRobots(newURL string) (bool, []string, []string, int) {
	defer timeTrack(time.Now(), "Process Robots")
	userAgent := false
	crawlDelay := 1
	disallowedPages := []string{}
	allowedPages := []string{}
	var buff bytes.Buffer

	URL, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	robotsTxt, err := ioutil.ReadFile(URL.Host)
	if err != nil {
		fmt.Print(err)
	}

	buff.WriteString(string(robotsTxt))
	buff.WriteString("\n")

	fileAsString := buff.String()

	robotLines := strings.Split(fileAsString, "\n")

	for _, lineValue := range robotLines {

		if lineValue == "" {
			continue
		}

		values := strings.Split(lineValue, ":")

		field := strings.TrimSpace(values[0])
		value := strings.TrimSpace(values[1])

		switch strings.ToLower(field) {
		case "user-agent":
			if value == "*" {
				userAgent = true
			}
			break
		case "disallow":
			disallowedPages = append(disallowedPages, value)
			break
		case "allow":
			allowedPages = append(allowedPages, value)
			break
		case "crawl-delay":
			crawlDelay, _ := strconv.Atoi(value)
			_ = crawlDelay
			break
		default:
			break
		}
	}
	return userAgent, disallowedPages, allowedPages, crawlDelay
}

func downloadAndCreateRobots(u *url.URL, constructURL string) {
	defer timeTrack(time.Now(), "Robots downloading and creating")
	output, err := os.Create(u.Host)
	if err != nil {
		fmt.Println("Error while creating", u.Host, "-", err)
	}
	defer output.Close()

	response, err := http.Get(constructURL)

	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	n, err := io.Copy(output, response.Body)
	if err != nil {
		fmt.Println("Error while downloading", u.Host, "-", err)
	}

	_ = n
}

func addToJSONMap(newURL string, jsonData []byte) string {
	defer timeTrack(time.Now(), "Add JSON to file")
	URL, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	var fileName = URL.Host + ".json"

	err = ioutil.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		panic(err)
	} else {
		return fileName
	}

}

func removeDuplicates(elements []string) []string {
	// takes inspiration from https://www.dotnetperls.com/duplicates-go but rather than int it removes duplicate strings
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for k, v := range elements {
		if encountered[elements[k]] == true || v == "" {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[k]] = true
			// Append to result slice.
			result = append(result, elements[k])
		}
	}
	// Return the new slice.
	return result
}

func normalise(newURL string) string {

	lowercaseURL := strings.ToLower(newURL)

	if !strings.HasSuffix(lowercaseURL, "/") {
		return newURL + "/"
	}
	return lowercaseURL

}

func fetchURL(baseURL string, c chan Document) {
	defer timeTrack(time.Now(), "Fetching URL, HTML parsing and search for assets")
	//normalise url
	Location := normalise(baseURL)

	doc, err := goquery.NewDocument(Location)
	if err != nil {
		var Urls []string
		var Stylesheets []string
		var Scripts []string
		var Images []string
		c <- Document{Location, Urls, Stylesheets, Scripts, Images}
		return
	}

	// Find all the anchor tags and get the href attribute value
	Urls := doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("href")

		// Parse the base url
		baseParse, err := url.Parse(Location)
		if err != nil {
			panic(err)
		}

		// Parse the context url
		contextParse, err := url.Parse(val)
		if err != nil {
			return ""
		}
		// remove encoded query values of context
		contextParse.RawQuery = ""
		// remove fragments of context
		contextParse.Fragment = ""
		// only return link if host of the base url and context url are the same
		if baseParse.Host == contextParse.Host {
			return contextParse.String()
		}
		return ""

	})

	// Find all the img tags and get the src attribute value
	Images := doc.Find("img[src]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("src")

		if strings.HasPrefix(val, "//") {
			return strings.Replace(val, "//", "http://", -1)
		}
		return val

	})

	// Find all the script tags and get the src attribute value and remove // if present and replace with http
	Scripts := doc.Find("script[src]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("src")
		if strings.HasPrefix(val, "//") {
			return strings.Replace(val, "//", "http://", -1)
		}

		return val
	})

	// Find all the link tags and get the href attribute value but only if the rel equals stylesheet
	Stylesheets := doc.Find("link[rel=stylesheet]").Map(func(_ int, s *goquery.Selection) string {
		val, _ := s.Attr("href")
		return val
	})

	Urls = removeDuplicates(Urls)
	Images = removeDuplicates(Images)
	Stylesheets = removeDuplicates(Stylesheets)
	Scripts = removeDuplicates(Scripts)

	newDocument := Document{Location, Urls, Stylesheets, Scripts, Images}

	c <- newDocument

}

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func processCrawler(
	documentMap []Document,
	channelDocument chan Document,
	queue []string,
	markedPages map[string]bool,
	disallowedPages []string,
	allowedPages []string,
	crawlDelay int,
	timeCalled time.Time,
) []Document {

	if len(queue) == 0 {
		return documentMap
	}

	currentTime := time.Now()
	diff := timeCalled.Sub(currentTime)
	transformedDiff := math.Abs(float64(diff.Seconds()))
	if transformedDiff < float64(crawlDelay) {
		delaySecond(float64(crawlDelay) + diff.Seconds())
	} else {
		log.Println("No delay needed")
	}

	timeCalled = time.Now()

	for _, URL := range queue {
		allowed := checkRobots(URL, disallowedPages)

		if allowed {
			go fetchURL(URL, channelDocument)
			queue = remove(queue, 0)

			details := <-channelDocument
			documentMap = append(documentMap, details)

			for _, URL := range details.Urls {

				// check the URL is actually a URL and if not continue to the next item in the array
				_, err := url.ParseRequestURI(URL)
				if err != nil {
					continue
				}

				//ensure value isn't empty or already in the system
				if !markedPages[URL] && URL != "" {
					markedPages[URL] = true
					queue = append(queue, URL)
				} else {

				}
			}

			log.Println("Queue length:", len(queue))
			log.Println("Documents length:", len(documentMap))

			return processCrawler(documentMap,
				channelDocument,
				queue,
				markedPages,
				disallowedPages,
				allowedPages,
				crawlDelay,
				timeCalled,
			)
		}
		fmt.Print("This page is not allowed to be crawled")
		continue
	}
	return documentMap
}

func startCrawler() {
	passedURL := os.Getenv("WEBSITE")

	_, err := url.ParseRequestURI(passedURL)
	if err != nil {
		panic(err)
	}

	channelDocument := make(chan Document) // channel for document
	newURL := normalise(passedURL)
	initialiseRobots(newURL)
	userAgent, disallowedPages, allowedPages, crawlDelay := processRobots(newURL)

	if userAgent {
		markedPages := make(map[string]bool) // URLs already in the system whether processed or in queue.
		var queue []string                   // URLs to be fetched, using a very simple array FIFO
		var documentMap []Document           // map of all the processed pages
		markedPages[newURL] = true
		queue = append(queue, newURL)
		timeCalled := time.Now()
		results := processCrawler(documentMap, channelDocument, queue, markedPages, disallowedPages, allowedPages, crawlDelay, timeCalled)
		b, err := json.MarshalIndent(results, "", "  ")
		fileName := addToJSONMap(newURL, b)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Sitemap: ", fileName)

	} else {
		fmt.Println("You're not allowed to crawl this site")
		return
	}
}

func main() {
	startCrawler()
}
