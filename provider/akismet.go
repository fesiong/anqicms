package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

const (
	AkismetApiURL = "https://rest.akismet.com/"

	CheckTypeGuestbook = 1 // 默认检查留言 0|1
	CheckTypeComment   = 2 // 检查评论
)

// HTTPClient is a interface for http client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type AkismetClient struct {
	// HTTPClient is underlying http client.
	// If it is nil, http.DefaultClient is used.
	HTTPClient HTTPClient

	// UserAgent is User-Agent header of requests.
	UserAgent string

	// APIKey is an API Key for using Akismet API.
	APIKey string

	// BaseURL is the endpoint of Akismet API.
	// If is is empty, https://rest.akismet.com/ is used.
	BaseURL string

	// support types
	CheckType []int
}

func (w *Website) InitAkismet() {
	setting := w.GetAkismetSetting()
	if setting.Open == false {
		w.AkismetClient = nil
		return
	}

	c := &AkismetClient{
		APIKey:    setting.ApiKey,
		CheckType: setting.CheckType,
	}

	if err := c.VerifyKey(context.Background(), w.System.BaseUrl); err != nil {
		log.Printf("Akismet api key verify error: %v", err)
		w.AkismetClient = nil
		return
	}

	w.AkismetClient = c
}

// AkismentCheck 垃圾内容检测
// checkType = 1，2; return status, ok ;status = 1 正常,2 垃圾;ok = true, false,false 表示检测失败或未检测
// 检测失败或未检测都返回 1, false
func (w *Website) AkismentCheck(ctx iris.Context, checkType int, data interface{}) (int, bool) {
	if w.AkismetClient == nil {
		return 1, false
	}
	// 没有来路
	//if !ctx.IsAjax() && ctx.Request().Referer() == "" {
	//	return 2, true
	//}
	//// 非标准单词
	//if checkType == CheckTypeGuestbook {
	//	guestbook, ok := data.(*model.Guestbook)
	//	if ok {
	//		var checkData []string
	//		if guestbook.Contact != "" && CheckContentIsEnglish(guestbook.Contact) {
	//			checkData = append(checkData, guestbook.Contact)
	//		}
	//		if guestbook.Content != "" && CheckContentIsEnglish(guestbook.Content) {
	//			checkData = append(checkData, guestbook.Content)
	//		}
	//		if guestbook.UserName != "" && CheckContentIsEnglish(guestbook.UserName) {
	//			checkData = append(checkData, guestbook.UserName)
	//		}
	//		for _, v := range checkData {
	//			// 纯数字的不检测
	//			matched, _ := regexp.MatchString(`^[0-9\s\-]+$`, v)
	//			if matched {
	//				continue
	//			}
	//			// 大写字母占比过大
	//			upperCount := 0
	//			for _, vv := range v {
	//				if unicode.IsUpper(vv) {
	//					upperCount++
	//				}
	//			}
	//			upperPercent := float64(upperCount) / float64(len(v))
	//			if upperPercent > 0.3 && upperPercent < 0.9 {
	//				return 2, true
	//			}
	//			// 未包含元音
	//			if !strings.ContainsAny(v, "aeiouAEIOU") {
	//				return 2, true
	//			}
	//		}
	//	}
	//}
	// end
	var needCheck = func(checkType int) bool {
		if checkType == CheckTypeGuestbook && len(w.AkismetClient.CheckType) == 0 {
			return true
		}
		for _, v := range w.AkismetClient.CheckType {
			if v == checkType {
				return true
			}
		}
		return false
	}
	ok := needCheck(checkType)
	if !ok {
		return 1, false
	}
	var akiComment = &AkismentComment{
		Blog:                w.System.BaseUrl,
		UserIP:              ctx.RemoteAddr(),
		UserAgent:           ctx.GetHeader("User-Agent"),
		Referrer:            ctx.Request().Referer(),
		Permalink:           ctx.FullRequestURI(),
		CommentDate:         time.Now(),
		CommentPostModified: time.Now(),
		BlogLang:            w.System.Language,
		BlogCharset:         "UTF-8",
	}
	if checkType == CheckTypeComment {
		akiComment.CommentType = CommentTypeComment
		comment, ok := data.(*model.Comment)
		if ok {
			akiComment.CommentAuthor = comment.UserName
			akiComment.CommentContent = comment.Content
			akiComment.CommentAuthorEmail = comment.Email
			if comment.UserId > 0 {
				user, err := w.GetUserInfoById(comment.UserId)
				if err == nil {
					akiComment.CommentAuthorEmail = user.Email
					akiComment.CommentAuthorURL = user.FullAvatarURL
				}
			}
		}
	} else {
		akiComment.CommentType = CommentTypeContactForm
		contact, ok := data.(*model.Guestbook)
		if ok {
			email := contact.Contact
			if !w.VerifyEmailFormat(email) {
				email = ""
				for _, v := range contact.ExtraData {
					vv, _ := v.(string)
					if w.VerifyEmailFormat(vv) {
						email = vv
						break
					}
				}
			}
			akiComment.CommentAuthor = contact.UserName
			akiComment.CommentAuthorEmail = email
			akiComment.CommentContent = contact.Content
		}
	}
	result, err := w.AkismetClient.CheckComment(ctx, akiComment)
	if err != nil {
		return 1, false
	}

	status := 1
	if result.Spam {
		status = 2
	}
	return status, true
}

func (c *AkismetClient) VerifyKey(ctx context.Context, blog string) error {
	// build the request.
	u, err := c.resolvePath("1.1/verify-key")
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("api_key", c.APIKey)
	form.Set("blog", blog)
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// send the request.
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// parse the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("akismet: unexpected status code: %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	respBody = bytes.TrimSpace(respBody)
	if string(respBody) != "valid" {
		return fmt.Errorf("akismet: your api key is %s", respBody)
	}

	return nil
}

type AkismentComment struct {
	// Blog is The front page or home URL of the instance making the request.
	// For a blog or wiki this would be the front page. Note: Must be a full URI, including http://.
	Blog string

	// UserIP is IP address of the comment submitter.
	UserIP string

	// UserAgent is the user agent string of the web browser submitting the comment.
	// Typically the HTTP_USER_AGENT cgi variable. Not to be confused with the user agent of your Akismet library.
	UserAgent string

	// Referrer is the content of the HTTP_REFERER header should be sent here.
	Referrer string

	// Permalink is the full permanent URL of the entry the comment was submitted to.
	Permalink string

	// CommentType is a string that describes the type of content being sent.
	CommentType CommentType

	// CommentAuthor is name submitted with the comment.
	CommentAuthor string

	// CommentAuthorEmail is Email address submitted with the comment.
	CommentAuthorEmail string

	// CommentAuthorURL is URL submitted with comment.
	// Only send a URL that was manually entered by the user,
	// not an automatically generated URL like the user’s profile URL on your site.
	CommentAuthorURL string

	// CommentContent is the content that was submitted.
	CommentContent string

	// The UTC timestamp of the creation of the comment, in ISO 8601 format.
	// May be omitted for comment-check requests if the comment is sent to the API at the time it is created.
	CommentDate time.Time

	// CommentPostModified is the UTC timestamp of the publication time for the post, page or thread on which the comment was posted.
	CommentPostModified time.Time

	// BlogLang indicates the language(s) in use on the blog or site,
	// in ISO 639-1 format, comma-separated. A site with articles in English and French might use “en, fr_ca”.
	BlogLang string

	// BlogCharset is the character encoding for the form values included
	// in comment_* parameters, such as “UTF-8” or “ISO-8859-1”.
	BlogCharset string

	// UserRole is the user role of the user who submitted the comment.
	// This is an optional parameter. If you set it to “administrator”, Akismet will always return false.
	UserRole string

	// IsTest is an optional parameter. You can use it when submitting test queries to Akismet.
	IsTest bool

	RecheckReason string

	HoneypotFieldName string
}

type CommentType string

const (
	// CommentTypeComment is a blog comment.
	CommentTypeComment CommentType = "comment"

	// CommentTypeForumPost is a top-level forum post.
	CommentTypeForumPost CommentType = "forum-post"

	// CommentTypeReply is reply to a top-level forum post.
	CommentTypeReply CommentType = "reply"

	// CommentTypeBlogPost is a blog post.
	CommentTypeBlogPost CommentType = "blog-post"

	// CommentTypeContactForm is a contact form or feedback form submission.
	CommentTypeContactForm CommentType = "contact-form"

	// CommentTypeSignUp is new user account.
	CommentTypeSignUp CommentType = "signup"

	// CommentTypeMessage is a message sent between just a few users.
	CommentTypeMessage CommentType = "message"
)

type Result struct {
	Spam bool
}

func (c *AkismetClient) CheckComment(ctx context.Context, comment *AkismentComment) (*Result, error) {
	// build the request.
	u, err := c.resolvePath("1.1/comment-check")
	if err != nil {
		return nil, err
	}
	form := c.buildCommentForm(comment)
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// send the request.
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// parse the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("akismet: unexpected status code: %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respBody = bytes.TrimSpace(respBody)
	if string(respBody) == "true" {
		return &Result{
			Spam: true,
		}, nil
	}
	if string(respBody) == "false" {
		return &Result{
			Spam: false,
		}, nil
	}

	return nil, fmt.Errorf("akismet: error from the server: %s", respBody)
}

// SubmitHam submits false-positives - items that were incorrectly classified as spam by Akismet.
func (c *AkismetClient) SubmitHam(ctx context.Context, comment *AkismentComment) error {
	// build the request.
	u, err := c.resolvePath("1.1/submit-ham")
	if err != nil {
		return err
	}
	form := c.buildCommentForm(comment)
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// send the request.
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// parse the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("akismet: unexpected status code: %d", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// SubmitSpam submits comments that weren’t marked as spam but should have been.
func (c *AkismetClient) SubmitSpam(ctx context.Context, comment *AkismentComment) error {
	// build the request.
	u, err := c.resolvePath("1.1/submit-spam")
	if err != nil {
		return err
	}
	form := c.buildCommentForm(comment)
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// send the request.
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// parse the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("akismet: unexpected status code: %d", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *AkismetClient) resolvePath(path string) (*url.URL, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = AkismetApiURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return base.JoinPath(path), nil
}

func (c *AkismetClient) do(req *http.Request) (*http.Response, error) {
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	} else {
		req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", "AnQiCMS", config.Version))
	}
	if c.HTTPClient == nil {
		return http.DefaultClient.Do(req)
	}
	return c.HTTPClient.Do(req)
}

func (c AkismetClient) buildCommentForm(comment *AkismentComment) url.Values {
	form := url.Values{}
	form.Set("api_key", c.APIKey)
	form.Set("blog", comment.Blog)
	form.Set("user_ip", comment.UserIP)
	if comment.UserAgent != "" {
		form.Set("user_agent", comment.UserAgent)
	}
	if comment.Referrer != "" {
		form.Set("referrer", comment.Referrer)
	}
	if comment.Permalink != "" {
		form.Set("permalink", comment.Permalink)
	}
	if comment.CommentType != "" {
		form.Set("comment_type", string(comment.CommentType))
	}
	if comment.CommentAuthor != "" {
		form.Set("comment_author", comment.CommentAuthor)
	}
	if comment.CommentAuthorEmail != "" {
		form.Set("comment_author_email", comment.CommentAuthorEmail)
	}
	if comment.CommentAuthorURL != "" {
		form.Set("comment_author_url", comment.CommentAuthorURL)
	}
	if comment.CommentContent != "" {
		form.Set("comment_content", comment.CommentContent)
	}
	if !comment.CommentDate.IsZero() {
		form.Set("comment_date_gmt", comment.CommentDate.UTC().Format(time.RFC3339))
	}
	if !comment.CommentPostModified.IsZero() {
		form.Set("comment_post_modified_gmt", comment.CommentPostModified.UTC().Format(time.RFC3339))
	}
	if comment.BlogLang != "" {
		form.Set("blog_lang", comment.BlogLang)
	}
	if comment.BlogCharset != "" {
		form.Set("blog_charset", comment.BlogCharset)
	}
	if comment.UserRole != "" {
		form.Set("user_role", comment.UserRole)
	}
	if comment.IsTest {
		form.Set("is_test", "1")
	}
	if comment.RecheckReason != "" {
		form.Set("recheck_reason", comment.RecheckReason)
	}
	if comment.HoneypotFieldName != "" {
		form.Set("honeypot_field_name", comment.HoneypotFieldName)
	}
	return form
}
