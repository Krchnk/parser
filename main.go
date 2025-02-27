package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

type Product struct {
	Name  string `json:"name"`
	Price string `json:"price"`
	URL   string `json:"url"`
}

func parseCategory(url string, urlSite string) ([]Product, error) {

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		chromedp.Flag("headless", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(1*time.Second),
		chromedp.WaitVisible(`[class*="ProductCard_root"]`),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		log.Printf("Ошибка при загрузке страницы: %v", err)
		os.WriteFile("debug.html", []byte(html), 0644)
		return nil, fmt.Errorf("ошибка при загрузке страницы: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("ошибка при парсинге HTML: %v", err)
	}

	var products []Product

	doc.Find(`[class*="ProductCard_root"]`).Each(func(i int, s *goquery.Selection) {
		name := s.Find(`[class*="ProductCard_name"]`).Text()
		priceElements := s.Find(`[class*="ProductCardActions_text"] span`)
		var price string
		if priceElements.Length() > 2 {
			secondPriceElement := priceElements.Eq(2)
			price = secondPriceElement.Text()
		} else {
			price = priceElements.Eq(0).Text()
		}

		re := regexp.MustCompile(`\d+`)
		price = re.FindString(price)

		parent := s.Parent()
		url := parent.AttrOr("href", "")
		url = fmt.Sprintf("https://%s%s", urlSite, url)

		name = strings.TrimSpace(name)

		products = append(products, Product{
			Name:  name,
			Price: price,
			URL:   url,
		})
	})

	return products, nil
}

func saveToCSV(products []Product, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("ошибка при создании файла: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	headers := []string{"Название", "Цена", "Ссылка"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("ошибка при записи заголовков: %v", err)
	}

	for _, product := range products {
		record := []string{product.Name, product.Price, product.URL}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("ошибка при записи данных: %v", err)
		}
	}

	return nil
}

func main() {
	siteFlag := flag.String("s", "", "сайт для ссылки")
	categoryFlag := flag.String("c", "", "Категория товара для ссылки")
	flag.Parse()

	if *categoryFlag == "" {
		fmt.Println("Ошибка: Не указана категория.")
		return
	}

	categoryURL := fmt.Sprintf("https://%s/category/%s", *siteFlag, *categoryFlag)

	products, err := parseCategory(categoryURL, *siteFlag)
	if err != nil {
		log.Fatalf("Ошибка при парсинге: %v", err)
	}

	filename := "products"
	filename = fmt.Sprintf("%s_%s.csv", filename, *categoryFlag)
	if err := saveToCSV(products, filename); err != nil {
		log.Fatalf("Ошибка при сохранении в CSV: %v", err)
	}

	fmt.Printf("Данные успешно сохранены в файл: %s\n", filename)
}
