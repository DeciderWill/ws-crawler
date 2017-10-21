package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Document : Object for each page with an array for each asset
type Document struct {
	Location    string
	Urls        []string
	Stylesheets []string
	Scripts     []string
	Images      []string
}

func downloadRobots(newURL string) {
	u, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	var needRefresh = false
	var constructURL = u.Scheme + "://" + u.Host + "/robots.txt"

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

func checkRobots(newURL string) bool {
	// This function checks for politeness regarding user agents, allow/deny and crawler rate
	var allowedScan = true
	var userAgent = false

	u, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	file, err := os.Open(u.Host)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		if scanner.Text() == "Allow: "+u.Path {
			allowedScan = true
		}

		if scanner.Text() == "Disallow: "+u.Path {
			allowedScan = false
		}

		if scanner.Text() == "User-agent: *" {
			userAgent = true
		}

	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if allowedScan && userAgent {
		return true
	}
	return false

}

func downloadAndCreateRobots(u *url.URL, constructURL string) {
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

func downloadAndCreateJSONMap(newURL string, jsonData []byte) string {
	u, err := url.Parse(newURL)
	if err != nil {
		panic(err)
	}

	var fileName = u.Host + ".json"

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

	var lowercaseURL = strings.ToLower(newURL)

	if !strings.HasSuffix(lowercaseURL, "/") {
		return newURL + "/"
	}
	return lowercaseURL

}

func fetchURL(baseURL string, c chan Document) {
	//normalise url
	var Location = normalise(baseURL)

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
			panic(err)
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

func processCrawler(documentMap map[string]Document, channelDocument chan Document, queue []string, markedPages map[string]bool) map[string]Document {
	if len(queue) == 0 {
		return documentMap
	}
	for k, v := range queue {
		_ = k
		var allowed = checkRobots(v)

		if allowed {
			go fetchURL(v, channelDocument)

			var details = <-channelDocument
			documentMap[details.Location] = details

			for k, v := range details.Urls {
				_ = k
				//ensure value isn't empty
				if !markedPages[v] && v != "" {
					markedPages[v] = true
					queue = append(queue, v)
				} else {

				}
			}

			queue = remove(queue, 0)

			log.Println("Queue length:", len(queue))
			log.Println("Documents length:", len(documentMap))

			return processCrawler(documentMap, channelDocument, queue, markedPages)
		}
		fmt.Print("Not allowed")
	}
	return documentMap
}

func main() {
	var passedURL string
	fmt.Print("Enter site to crawl: ")
	fmt.Scanln(&passedURL)

	u, err := url.ParseRequestURI(passedURL)
	if err != nil {
		panic(err)
	}

	_ = u

	channelDocument := make(chan Document) // channel for document
	var newURL = normalise(passedURL)
	downloadRobots(newURL)
	var markedPages = make(map[string]bool)     // URLs already in the system whether processed or in queue.
	var queue []string                          // URLs to be fetched, using a very simple array FIFO
	var documentMap = make(map[string]Document) // map of all the processed pages
	markedPages[newURL] = true
	queue = append(queue, newURL)
	var results = processCrawler(documentMap, channelDocument, queue, markedPages)
	b, err := json.MarshalIndent(results, "", "  ")
	var fileName = downloadAndCreateJSONMap(newURL, b)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Sitemap: ", fileName)

}
