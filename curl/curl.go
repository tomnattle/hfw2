package curl

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	neturl "net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/axgle/mahonia"
)

type Response struct {
	Headers    map[string]string `json:"headers"`
	Cookie     string            `json:"cookie"`
	Url        string            `json:"url"`
	FollowUrls []string          `json:"follow_urls"`
	Body       string            `json:"body"`
}

var stopRedirect = errors.New("no redirects allowed")

type Curl struct {
	Url, Method, PostFields, Cookie, Referer string

	Headers map[string]string
	Options map[string]bool

	Timeout time.Duration

	RedirectCount int

	followUrls []string
}

var tr = &http.Transport{
	Dial: (&net.Dialer{
		Timeout:   4 * time.Second,
		KeepAlive: 3600 * time.Second,
	}).Dial,
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	// DisableKeepAlives:     true,
	TLSHandshakeTimeout:   4 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
}

func NewCurl(url string) *Curl {
	return &Curl{
		Headers:    make(map[string]string),
		Options:    make(map[string]bool),
		followUrls: make([]string, 0),
		Url:        url,
	}
}

func (curls *Curl) SetHeaders(headers map[string]string) {
	curls.Headers["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	curls.Headers["Accept-Encoding"] = "gzip, deflate"
	curls.Headers["Accept-Language"] = "zh-cn,zh;q=0.8,en-us;q=0.5,en;q=0.3"
	// curls.Headers["Connection"] = "close"
	curls.Headers["Connection"] = "keep-alive"
	curls.Headers["User-Agent"] = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/65.0.3325.146 Safari/537.36"

	//使用这个header是因为避免100的状态码
	curls.Headers["Expect"] = ""

	for k, v := range headers {
		curls.Headers[k] = v
	}
}

func (curls *Curl) SetOptions(options map[string]bool) {
	for k, v := range options {
		curls.Options[k] = v
	}
}

func (curls *Curl) Request() (rs Response, err error) {
	if len(curls.Options) == 0 && len(curls.Headers) == 0 {
		rs, err = curls.SimpleGet()
	} else {
		rs, err = curls.Post()
	}

	return
}

//不设置header、cookie的请求
func (curls *Curl) SimpleGet() (response Response, err error) {
	var resp *http.Response
	resp, err = http.Get(curls.Url)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return curls.curlResponse(resp)
}

func (curls *Curl) Post() (rs Response, err error) {
	var httprequest *http.Request
	var httpclient *http.Client
	var httpresponse *http.Response

	httpclient = &http.Client{tr, func(_ *http.Request, via []*http.Request) error { return stopRedirect }, nil, curls.Timeout}

	start_time := time.Now().UnixNano()
	num := 0
	for {
		if "" != curls.PostFields || curls.Method == "post" {
			httprequest, _ = curls.postForm()
		} else {
			httprequest, _ = http.NewRequest("GET", curls.Url, nil)
		}

		// curls.hosttoip(httprequest)

		if curls.Headers != nil {
			for key, value := range curls.Headers {
				httprequest.Header.Add(key, value)
			}
		}

		if curls.Cookie != "" {
			httprequest.Header.Add("Cookie", curls.Cookie)
		}
		if curls.Referer != "" {
			httprequest.Header.Add("Referer", curls.Referer)
		}

		//使用过一次后，post的内容被读走了，直接重试执行Do无用
		httpresponse, err = httpclient.Do(httprequest)
		if nil != err {
			//不是重定向里抛出的错误
			if urlError, ok := err.(*neturl.Error); !ok || urlError.Err != stopRedirect {
				if time.Now().UnixNano()-start_time >= int64(2*time.Second) {
					return rs, err
				}
				num++
				if num == 5 {
					//TODO
					// RestartLink("重试达到5次")
				}
				continue
			}
		}
		break
	}
	defer func() {
		_ = httpresponse.Body.Close()
	}()

	return curls.curlResponse(httpresponse)
}

func (curls *Curl) postForm() (httprequest *http.Request, err error) {
	if curls.Headers["Content-Type"] == "multipart/form-data" {
		postValues, err := neturl.ParseQuery(curls.PostFields)
		if err != nil {
			return nil, err
		}

		var b bytes.Buffer
		w := multipart.NewWriter(&b)

		for k, _ := range postValues {
			_ = w.WriteField(k, postValues.Get(k))
		}
		_ = w.Close()
		httprequest, _ = http.NewRequest("POST", curls.Url, &b)
		httprequest.Header.Add("Content-Type", w.FormDataContentType())
	} else {
		b := strings.NewReader(curls.PostFields)
		httprequest, _ = http.NewRequest("POST", curls.Url, b)
		httprequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	delete(curls.Headers, "Content-Type")

	return httprequest, nil
}

//处理获取的页面
func (curls *Curl) curlResponse(resp *http.Response) (response Response, err error) {
	response.Body, err = curls.getBody(resp)
	if err != nil {
		return
	}
	response.Headers = curls.rcHeader(resp.Header)

	location, _ := resp.Location()
	if nil != location {
		location_url := location.String()
		response.Headers["Location"] = location_url

		//如果不自动重定向，就直接返回
		if curls.Options["redirect"] {
			if curls.RedirectCount < 5 {
				curls.Referer = curls.Url
				curls.RedirectCount++
				curls.followUrls = append(curls.followUrls, curls.Url)
				curls.Url = location_url
				curls.Method = "get"
				curls.PostFields = ""
				curls.Cookie = curls.afterCookie(resp)

				return curls.Post()
			} else {
				return response, errors.New("重定向次数过多")
			}
		}
	}

	response.Headers["Status"] = resp.Status
	response.Headers["Status-Code"] = strconv.Itoa(resp.StatusCode)
	response.Headers["Proto"] = resp.Proto
	response.Cookie = curls.afterCookie(resp)
	response.Url = curls.Url
	response.FollowUrls = curls.followUrls

	return response, nil
}

func (curls *Curl) getBody(resp *http.Response) (utf8body string, err error) {
	var (
		body []byte
	)

	//如果出现302或301，已经表示是不自动重定向 或者出现200才读
	if resp.StatusCode == http.StatusOK || resp.StatusCode == 299 {
		if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return "", err
			}
			body, err = ioutil.ReadAll(reader)
		} else if strings.Contains(resp.Header.Get("Content-Encoding"), "deflate") {
			reader := flate.NewReader(resp.Body)
			defer func() {
				_ = reader.Close()
			}()
			body, err = ioutil.ReadAll(reader)
		} else {
			body, err = ioutil.ReadAll(resp.Body)
		}
		if err != nil {
			return
		}
	}

	return strings.TrimSpace(string(body)), nil
}

func (curl *Curl) toUtf8(body string) string {
	if utf8.ValidString(body) == false {
		enc := mahonia.NewDecoder("gb18030")
		body = enc.ConvertString(body)
	}

	return body
}

//返回结果的时候，转换cookie为字符串
func (curls *Curl) afterCookie(resp *http.Response) string {
	//去掉重复
	rs_tmp := make(map[string]string)

	//先处理传进来的cookie
	if curls.Cookie != "" {
		tmp := strings.Split(curls.Cookie, "; ")
		for _, v := range tmp {
			tmp_one := strings.SplitN(v, "=", 2)
			rs_tmp[tmp_one[0]] = tmp_one[1]
		}
	}

	//处理新cookie
	for _, v := range resp.Cookies() {
		//过期
		if v.Value == "EXPIRED" {
			delete(rs_tmp, v.Name)
			continue
		}
		rs_tmp[v.Name] = v.Value
	}
	//用于join
	rs := make([]string, len(rs_tmp))
	i := 0
	for k, v := range rs_tmp {
		rs[i] = k + "=" + v
		i++
	}

	sort.Strings(rs)

	return strings.TrimSpace(strings.Join(rs, "; "))
}

//整理header
func (curls *Curl) rcHeader(header map[string][]string) map[string]string {
	headers := make(map[string]string, len(header))
	for k, v := range header {
		headers[k] = strings.Join(v, "\n")
	}

	return headers
}