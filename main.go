package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

type Request struct {
	url   string
	depth int
}

type Result struct {
	err error
	url string
}

type Document struct {
	ItemNumber  string
	Name        string
	ObjectClass string
	Tags        []string
	Author      string
	Caption     string
}

func InitFirestore() (*firestore.Client, context.Context) {
	ctx := context.Background()
	sa := option.WithCredentialsFile("/Users/yuichiroutakahashi/Documents/hobby/go/serviceAccount.json")

	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	return client, ctx
}

func Fetch(u string) (urls []string, titles []string, err error) {
	baseURL, err := url.Parse(u)
	if err != nil {
		return
	}

	resp, err := http.Get(baseURL.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var r io.Reader = resp.Body
	// r = io.TeeReader(r, os.Stderr)

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return
	}

	urls = make([]string, 0)
	titles = make([]string, 0)
	// regex, regexErr := regexp.Compile(`http://scp-jp\.wikidot\.com/scp-[0-9]{1,}-jp`)
	regex, regexErr := regexp.Compile(`http://scp-jp\.wikidot\.com/scp-[0-9]{1,}`)
	if regexErr != nil {
		fmt.Println("regex comple error:")
		fmt.Println(err)
		return
	}

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exsits := s.Attr("href")
		if exsits {
			reqURL, err := baseURL.Parse(href)

			if err == nil && regex.MatchString(reqURL.String()) {
				strs := strings.Split(s.Parent().Text(), "-")
				title := strs[len(strs)-1]
				urls = append(urls, reqURL.String())
				titles = append(titles, strings.TrimSpace(title))
			}
		}
	})

	return
}

func Crawl(url string, depth int) {
	urls, titles, err := Fetch(url)
	client, ctx := InitFirestore()
	if err != nil {
		str := fmt.Sprintf("error: %s", err)
		fmt.Println(str)
		return
	}

	fmt.Println("Finished")
	fmt.Println("urls count: " + strconv.Itoa(len(urls)))
	fmt.Println("titles count: " + strconv.Itoa(len(titles)))

	// fmt.Printf("titles: %s", titles)

	for i := 0; i < len(urls); i++ {
		url := urls[i]
		title := titles[i]

		fmt.Println("start fetching...: " + title)
		resp, err := http.Get(url)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		doc, err := Extract(resp.Body, url, title)
		if err != nil {
			fmt.Println(err)
			continue
		}

		client.Collection("scp").Doc(doc.ItemNumber).Set(ctx, map[string]interface{}{
			"itemNumber":  doc.ItemNumber,
			"name":        doc.Name,
			"objectClass": doc.ObjectClass,
			"tags":        doc.Tags,
			"author":      doc.Author,
			"caption":     doc.Caption,
			"url":         url,
		})

		fmt.Printf("%+v\n", doc)
		fmt.Println("fetching finished: " + title)
	}
}

func Extract(r io.Reader, pageURL string, title string) (Document, error) {
	// r = io.TeeReader(r, os.Stderr)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		fmt.Println("[GOQUERY ERROR] document create failed:")
		fmt.Println(err)
		return Document{}, err
	}

	itemNumber := strings.TrimSpace(doc.Find("#page-title").Text())

	author := ""
	doc.Find("span.printuser").Each(func(_ int, s *goquery.Selection) {
		author = strings.TrimSpace(s.Find("a").Last().Text())
		if author == "" {
			author = "-"
		}
		// fmt.Println("author: " + author)
	})

	tags := []string{}
	doc.Find(".page-tags").Find("a").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		// fmt.Println("tag: " + text)
		tags = append(tags, text)
	})

	content := doc.Find("#page-content")
	objectClass := ""
	caption := ""
	content.Find("p").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()

		if strings.Contains(text, "オブジェクトクラス:") {
			// fmt.Println(text)
			objectClass = strings.Split(text, ":")[1]
			objectClass = strings.TrimSpace(objectClass)
		}

		if strings.Contains(text, "説明:") {
			caption = strings.Split(text, ":")[1]
			caption = strings.TrimSpace(caption)
		}
	})

	return Document{itemNumber, title, objectClass, tags, author, caption}, nil
}

func AddURL(urls []string, client *firestore.Client, ctx context.Context) {
	regex, err := regexp.Compile(`http://scp-jp\.wikidot\.com/(scp-[0-9]{1,})`)
	if err != nil {
		fmt.Printf("regex error: %s", err)
		return
	}

	itemNums := regex.FindAllStringSubmatch(urls[1], -1)
	num := strings.ToUpper(itemNums[0][1])
	fmt.Printf("scp number: %s", num)
	client.Collection("scp").Doc(num).Update(ctx, []firestore.Update{{Path: "url", Value: urls[0]}})
	// client.Collection("scp").Doc(num).Set(ctx, map[string]interface{}{
	// 	"url": urls[0],
	// }, firestore.MergeAll)

	scp, err2 := client.Collection("scp").Doc(num).Get(ctx)
	if err2 != nil {
		// fmt.Printf("error: %s", err2)
		fmt.Printf("is Same: %s", "SCP-001" == num)
	}
	fmt.Printf("scp-001: %s", scp.Data())
}

const crawlerDepthDefault = 5

var crawlerDepth int

func main() {
	flag.IntVar(&crawlerDepth, "depth", crawlerDepthDefault, "クロールする深さを指定。")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "URLを指定してください。")
		os.Exit(1)
	}

	startURL := flag.Arg(0)
	if crawlerDepth < 1 {
		crawlerDepth = crawlerDepthDefault
	}

	req := Request{
		url:   startURL,
		depth: crawlerDepth,
	}

	Crawl(req.url, req.depth)
}
