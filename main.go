package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// 下载m3u8文件
func downloadM3U8(url string) (string, error) {
	baseName := filepath.Base(url)
	dir := strings.TrimSuffix(baseName, ".m3u8")
	err := os.Mkdir(dir, 0755)
	if err != nil && !os.IsExist(err) {
		return "", err
	}
	err = os.Chdir(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("wget", url)
	err = cmd.Run()
	if err != nil {
		return "", err
	}
	return baseName, nil
}

// 获取ts下载文件
func getUrls(prefix, filename string) ([]byte, []string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, nil
	}

	keyUrlRE := regexp.MustCompile(`AES-128,URI="(.*?)"`)
	keyUrl := keyUrlRE.FindSubmatch(data)
	if len(keyUrl) < 2 {
		return nil, nil, errors.New("failed to match key url")
	}

	// 解析url
	tsUrlRE := regexp.MustCompile(`.+ts\?start=.+`)
	tsUrls := tsUrlRE.FindAllSubmatch(data, -1)
	if len(tsUrls) == 0 {
		return nil, nil, errors.New("failed to match ts urls")
	}

	urls := make([]string, len(tsUrls))
	for i, lv1 := range tsUrls {
		urls[i] = prefix + string(lv1[0])
	}

	cmd := exec.Command("wget", string(keyUrl[1]), "-O", "key")
	err = cmd.Run()
	if err != nil {
		return nil, nil, err
	}

	key, err := ioutil.ReadFile("./key")
	if err != nil {
		return nil, nil, err
	}

	return key, urls, nil
}

type task struct {
	num int
	url string
}

type result struct {
	num      int
	url      string
	filename string
	err      error
}

func downloadChunks(key []byte, urls []string) (int, error) {
	downloadCh := make(chan task, len(urls))
	resultCh := make(chan result, len(urls))
	var wg sync.WaitGroup
	wg.Add(len(urls))

	// 发出下载任务
	for i, url := range urls {
		downloadCh <- task{
			num: i,
			url: url,
		}
	}
	close(downloadCh)

	// 启动10个协程下载文件
	fmt.Println("total: ", len(urls))
	for i := 0; i < 10; i++ {
		go func() {
			for t := range downloadCh {
				// 下载文件
				fmt.Printf("downloading %d %s\n", t.num, t.url)
				filename := strconv.Itoa(t.num) + ".ts"
				cmd := exec.Command("wget", t.url, "-O", filename)
				err := cmd.Run()
				if err != nil {
					fmt.Println("error", err)
				}
				resultCh <- result{
					num:      t.num,
					url:      t.url,
					filename: filename,
					err:      err,
				}
				wg.Done()
			}
		}()
	}

	wg.Wait()

	// 检查结果
	fmt.Println(len(resultCh))
	for i := 0; i < len(urls); i++ {
		rs := <-resultCh
		if rs.err != nil {
			return 0, fmt.Errorf("failed to download %s: %v", rs.url, rs.err)
		}

		// 解密
		block, err := aes.NewCipher(key)
		if err != nil {
			return 0, err
		}

		fileData, err := ioutil.ReadFile(rs.filename)
		if err != nil {
			return 0, err
		}
		pt := make([]byte, len(fileData))

		bm := cipher.NewCBCDecrypter(block, bytes.Repeat([]byte{0}, 16))
		bm.CryptBlocks(pt, fileData)

		err = ioutil.WriteFile(rs.filename, pt, 0755)
		if err != nil {
			return 0, err
		}
	}

	return len(urls), nil
}

func mergeFile(count int) error {
	// ffmpeg -i "concat:ttt.ts|tt2.ts" -c copy output.ts
	files := make([]string, count)
	for i := range files {
		files[i] = strconv.Itoa(i) + ".ts"
	}

	// Too many open files
	// ulimit -n 1024

	fmt.Println("ffmpeg", "-i", fmt.Sprintf("\"concat:%s\"", strings.Join(files, "|")), "-c", "copy", "merge.ts")

	cmd := exec.Command("ffmpeg", "-i", fmt.Sprintf("concat:%s", strings.Join(files, "|")), "-c", "copy", "merge.ts")
	o, e := cmd.CombinedOutput()
	fmt.Println(string(o))
	return e
}

func getPrefix(url string) string {
	i := strings.LastIndex(url, "/")
	return url[:i+1]
}

func main() {
	var url string
	var newName string
	flag.StringVar(&url, "u", "", "m3u8 url")
	flag.StringVar(&newName, "n", "", "new name")
	flag.Parse()

	if !strings.HasSuffix(url, "m3u8") {
		fmt.Println("please enter valid m3u8 url")
		return
	}

	// 1. 下载m3u8
	filename, err := downloadM3U8(url)
	if err != nil {
		panic(err)
	}

	// 2. 解析出key和分片url
	key, tsUrls, err := getUrls(getPrefix(url), filename)
	if err != nil {
		panic(err)
	}

	// 3. 并发下载分片文件并解密
	count, err := downloadChunks(key, tsUrls)
	if err != nil {
		panic(err)
	}

	// 4. 合并文件
	err = mergeFile(count)
	fmt.Println(err)

	if err == nil {
		if len(newName) > 0 {
			err = os.Rename("merge.ts", "../"+newName+".ts")
		} else {
			err = os.Rename("merge.ts", "../merge.ts")
		}
		fmt.Println("move", err)
		if err == nil {
			os.Chdir("../")
			err = os.RemoveAll(strings.TrimSuffix(filepath.Base(url), ".m3u8"))
			fmt.Println("remove", err)
		}
	}
}
