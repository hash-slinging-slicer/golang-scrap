package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

func main() {
	// Create a new engine
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Get("/scrap/:url", func(c *fiber.Ctx) error {

		url := c.Params("url")

		// READ COOKIE
		filePath := "cookie.txt"
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return c.SendString("Error reading cookies")
		}
		cookieString := string(content)
		// END READ COOKIE

		htmlContent, err := fetchHTML("https://www.facebook.com/watch/?v="+url, cookieString)
		if err != nil {
			log.Fatal(err)
		}

		c.Set("Content-Type", "application/json")
		return c.JSON(htmlContent)
	})

	app.Get("/upload", func(c *fiber.Ctx) error {
		return c.Render("upload", fiber.Map{
			"Title": "Upload Kebenaran",
		})
	})

	app.Post("/upload-proses", func(c *fiber.Ctx) error {
		// Parse the form file
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Save the uploaded file to the server
		newFile := "cookie.txt"
		err = c.SaveFile(file, fmt.Sprintf("./%s", newFile))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "File uploaded successfully",
		})
	})

	app.Listen(":5000")
}

func SetCookie(name, value, domain, path string, httpOnly, secure bool) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// expr := cdp.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour))
		network.SetCookie(name, value).
			// WithExpires(&expr).
			WithDomain(domain).
			WithPath(path).
			WithHTTPOnly(httpOnly).
			WithSecure(secure).
			Do(ctx)
		return nil
	})
}

func fetchHTML(url string, cookieString string) (map[string][]string, error) {
	// Initialize a new browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Split the string into individual cookies
	cookiesSlice := strings.Split(cookieString, "; ")

	// Loop through each cookie and set it
	for _, cookie := range cookiesSlice {
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) == 2 {
			name := parts[0]
			value := parts[1]
			err := chromedp.Run(ctx, SetCookie(name, value, ".facebook.com", "/", false, false))
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	// Navigate to the URL and fetch the rendered HTML
	var htmlContent map[string][]string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Evaluate(`
		(function() {
			var scripts = Array.from(document.querySelectorAll('script[data-content-len]'));
			var urls = { hd: [], sd: [] };
			var urlFoundHd = false;
			var urlFoundSd = false;
	
			function findUrlInObject(obj, hdKey, sdKey) {
					for (var key in obj) {
							if (obj.hasOwnProperty(key)) {
									if (key === 'browser_native_hd_url') {
											urls.hd.push(obj[key]);
											urlFoundHd = true;
									} else if (key === 'browser_native_sd_url') {
											urls.sd.push(obj[key]);
											urlFoundSd = true;
									} else if (typeof obj[key] === 'object') {
											findUrlInObject(obj[key], hdKey, sdKey);
											if (urlFoundHd && urlFoundSd) {
													break;
											}
									}

									if(urlFoundHd && urlFoundSd){
										break;
									}
							}
					}
			}
	
			outerLoop: for (var i = 0; i < scripts.length; i++) {
					var script = scripts[i];
					var contentLen = parseInt(script.getAttribute('data-content-len'));
	
					if (!isNaN(contentLen) && contentLen > 10000) {
							var jsonContent = script.innerText;
	
							try {
									var jsonData = JSON.parse(jsonContent);
	
									if (typeof jsonData === 'object') {
											findUrlInObject(jsonData, 'hd_url', 'sd_url');
											if (urlFound) {
													break outerLoop;
											}
									}
							} catch (error) {
									console.error('Error parsing JSON:', error);
							}
					}
			}
	
			return urls;
	})();
	
		`, &htmlContent),
	)
	if err != nil {
		return htmlContent, err
	}

	return htmlContent, nil
}
