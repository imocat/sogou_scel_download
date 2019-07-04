package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func pathExists(path string) (exists bool, err error) {

	_, err = os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, nil
}

func existsCallback(filePath string, callback func(exists bool) (err error)) error {
	exists, err := pathExists(filePath)
	if err == nil {
		return callback(exists)
	}

	return err
}

func download(remoteUrl string, localPath string) (err error) {

	newFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer newFile.Close()

	clt := http.Client{Timeout: 300 * time.Second}
	resp, err := clt.Get(remoteUrl)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(newFile, resp.Body)
	return err
}

func getHtml(url string) (body []byte, err error) {

	clt := http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := clt.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("fail to get response: %s", string(body))
	}

	return
}

var cpuNum int = runtime.NumCPU()
var c = make(chan int, cpuNum)

func main() {

	var downloadDir string
	flag.StringVar(&downloadDir, "dir", "/tmp/cell", "download path")
	flag.Parse()

	runtime.GOMAXPROCS(cpuNum)

	//并发执行
	for i := 0; i < cpuNum; i++ {
		go work(downloadDir)
	}

	i := 1
	for {
		c <- i
		i++
	}

}

func work(downloadDir string) {

	for {
		categoryID := <-c
		findSCELURL(downloadDir, 100, categoryID)
	}

}

func downloadScelFile(downloadDir string, cellURL string, cellName string) (err error) {

	fmt.Println(cellName)

	cellPath := path.Join(downloadDir, strings.Replace(cellName, "/", "", -1)+".scel")
	err = existsCallback(cellPath, func(exists bool) (err error) {
		if exists {
			return
		}

		return download(cellURL, cellPath)
	})

	if err != nil {
		return err
	}

	return
}

func findSCELURL(downloadDir string, maxPages int, categoryID int) (nums int, err error) {

	err = existsCallback(downloadDir, func(exists bool) (err error) {
		if exists {
			return
		}
		return os.Mkdir(downloadDir, 0600)
	})

	if err != nil {
		return
	}

	//分类翻页数量
	for page := 1; page <= maxPages; page++ {

		url := fmt.Sprintf("https://pinyin.sogou.com/dict/cate/index/%d/default/%d", categoryID, page)
		body, err := getHtml(url)
		if err != nil {
			return nums, err
		}

		re, err := regexp.Compile(`/dict/detail/index/(\d+)`)
		if err != nil {
			return nums, err
		}

		results := re.FindAllStringSubmatch(string(body), -1)
		if err != nil {
			return nums, err
		}

		//fmt.Println(categoryID, page, len(results))

		nums = len(results)
		//翻页没有数据时，退出循环
		if nums == 0 {
			break
		}

		for i := range results {
			cellID, err := strconv.Atoi(results[i][1])
			if err != nil {
				return nums, err
			}

			body, err := getHtml(fmt.Sprintf("https://pinyin.sogou.com/dict/detail/index/%d", cellID))
			if err != nil {
				return nums, err
			}

			red, err := regexp.Compile(`\/\/pinyin\.sogou\.com\/d\/dict\/download_cell\.php\?id=(\d+)&name=([^"&]+)`)
			if err != nil {
				return nums, err
			}

			cellResults := red.FindAllStringSubmatch(string(body), -1)
			if err != nil {
				return nums, err
			}

			//没有数据，退出
			if len(cellResults) == 0 {
				break
			}

			for j := range cellResults {
				cellURL := fmt.Sprintf("http:%s", cellResults[j][0])
				cellName := cellResults[j][2]

				err = downloadScelFile(downloadDir, cellURL, cellName)
				if err != nil {
					return nums, err
				}
			}

		}

		//翻页列表数据少于10条，表示没有下一页，退出循环
		if nums < 10 {
			break
		}
	}

	return
}
