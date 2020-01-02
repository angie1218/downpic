package main

import (
	"fmt"
	"flag"
	"runtime"
	"log"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"io"
	"sync"
	"strings"
	"time"
	"strconv"
	"path"
	"os"
)

type NotFoundErrorStruct struct {
	Message string
}

func (e NotFoundErrorStruct) Error() string {
	return e.Message
}

type RemoteErrorStruct struct {
	Host string
	Err  error
}

func (e *RemoteErrorStruct) Error() string {
	return e.Err.Error()
}

type downPicHandler struct{
	client *http.Client
}

var downPic = downPicHandler{&http.Client{}}
var sw sync.WaitGroup

var pics chan string

func main() {
	runtime.NumCPU()

	pics = make(chan string, 200)
	baseUrl := "https://m.mei.com/a/%d"
	id := flag.Int("id", 0, "base url")
	pages := flag.Int("p", 0, "total pages")
	flag.Parse()
	if *id == 0 || *pages == 0 {
		log.Fatal("缺少请求参数")
	}
	for i := 1; i < *pages; i++ {
		log.Printf("抓取页面：%d", i)
		var url string
		if i == 1 {
			url = strings.Replace(baseUrl, "_%d", "", -1)
		} else {
			url = fmt.Sprintf(baseUrl, i)
		}
		start := time.Now()
		resp, err := downPic.HttpGet(url, nil)
		if err != nil {
			log.Fatalf("获取页面失败（%s）：%v", url, err)
		}
		html, err := goquery.NewDocumentFromReader(resp)
		if err != nil {
			log.Fatalf("解析页面失败（%s）：%v", url, err)
		}
		log.Fatalf("获取、 解析页面（%s）：%v 耗时 : %.2fs", url, time.Since(start).Seconds())
		picList := downPic.GetPics(html)
		if len(picList) > 0 {
			for _, pic := range picList{
				pics <- pic
				sw.Add(1)
				go downPic.DownloadPic()
			}
			sw.Wait()
		}
	}
}

func (h *downPicHandler) HttpGet(url string, header http.Header) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/29.0.1541.0 Safari/537.36")
	req.Header.Set("Referer", "https://m.mei.com/")
	for k, vs := range header {
		req.Header[k] = vs
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, &RemoteErrorStruct{req.URL.Host, err}
	}
	if resp.StatusCode == 200 {
		return resp.Body, nil
	}
	resp.Body.Close()
	if resp.StatusCode == 404 { // 403 can be rate limit error.  || resp.StatusCode == 403 {
		err = NotFoundErrorStruct{"Resource not found: " + url}
	} else {
		err = &RemoteErrorStruct{req.URL.Host, fmt.Errorf("get %s -> %d", url, resp.StatusCode)}
	}
	return nil, err
}

func (h *downPicHandler) GetPics(html *goquery.Document) []string {
	var picList []string
	html.Find("img[class=tupian_img]").Each(func(i int, selection *goquery.Selection) {
		url, _ := selection.Attr("src")
		picList = append(picList, url)
	})

	return picList
}

func (h *downPicHandler) DownloadPic() {
	url := <- pics
	if url != "" {
		log.Printf("正在下载：%s", url)
		fileName := "pictures/" + strconv.Itoa(time.Now().Nanosecond()) + "_" + path.Base(url)

		rc, err := h.HttpGet(url, nil)
		defer rc.Close()
		if err == nil {
			os.MkdirAll(path.Dir(fileName), os.ModePerm)
			f, err := os.Create(fileName)
			defer f.Close()
			if err == nil {
				_, err = io.Copy(f, rc)
				if err != nil {
					log.Printf("图片下载失败（%s）：%v", url, err)
				}
			}
		}
	}
	sw.Done()
}
