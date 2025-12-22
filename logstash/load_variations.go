package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ProductSeries represents the top level product series
type ProductSeries struct {
	XMLName       xml.Name      `xml:"productSeries"`
	ProductSeries []ProductBase `xml:"productSeries"`
}

// ProductBase represents a base product
type ProductBase struct {
	Code         string     `xml:"code,attr"`
	Products     Products   `xml:"products"`
}

// Products contains product variations
type Products struct {
	Product []Variation `xml:"product"`
}

// Variation represents a product variation
type Variation struct {
	PartNumber       string       `xml:"partNumber,attr"`
	LastChange       string       `xml:"lastChange,attr"`
	ImageUrl         string       `xml:"imageUrl"`
	ThumbnailUrl     string       `xml:"thumbnailUrl,omitempty"`
	DatasheetUrl     string       `xml:"datasheetUrl"`
	StepFileUrl      string       `xml:"stepFileUrl,omitempty"`
	Nrnd             string       `xml:"nrnd,attr"`
	NrndStart        string       `xml:"nrndStart"`
	EndOfLife        string       `xml:"endOfLife,attr"`
	EndOfLifeStart   string       `xml:"endOfLifeStart"`
	AlternativeProductSeries string `xml:"alternativeProductSeries"`
	AlternativeProduct string     `xml:"alternativeProduct"`
	ComingSoon       string       `xml:"comingSoon,attr"`
	New              string       `xml:"new,attr"`
	Focus            string       `xml:"focus,attr"`
	Replaces         []Replaces   `xml:"replaces"`
	Features         Features     `xml:"features"`
}

// Replaces represents replacement products
type Replaces struct {
	Text string `xml:",chardata"`
}

// Features contains all types of features
type Features struct {
	NumericFeature []NumericFeature `xml:"numericFeature"`
	SetFeature     []SetFeature     `xml:"setFeature"`
	RangeFeature   []RangeFeature   `xml:"rangeFeature"`
	EnumFeature    []EnumFeature    `xml:"enumFeature"`
	BooleanFeature []BooleanFeature `xml:"booleanFeature"`
	DayFeature     []DayFeature     `xml:"dayFeature"`
	StringFeature  []StringFeature  `xml:"stringFeature"`
}

// NumericFeature represents numeric feature
type NumericFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// EnumFeature represents enum feature
type EnumFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// RangeFeature represents range feature
type RangeFeature struct {
	ID   string `xml:"id,attr"`
	From string `xml:"from,attr"`
	To   string `xml:"to,attr"`
}

// SetFeature represents set feature
type SetFeature struct {
	ID    string   `xml:"id,attr"`
	Value []string `xml:"value"`
}

// BooleanFeature represents boolean feature
type BooleanFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// DayFeature represents day feature
type DayFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// StringFeature represents string feature
type StringFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// CatalogVariation represents a variation in our internal structure
type CatalogVariation struct {
	ProductCode             string            `json:"productCode"`
	VariationCode           string            `json:"variationCode"`
	LastChange              int64             `json:"lastChange"`
	ImageUrl                string            `json:"imageUrl"`
	ThumbnailUrl            string            `json:"thumbnailUrl"`
	DatasheetUrl            string            `json:"datasheetUrl"`
	StepFileUrl             string            `json:"stepFileUrl"`
	Nrnd                    int               `json:"nrnd"`
	NrndStart               string            `json:"nrndStart"`
	EndOfLife               int               `json:"endOfLife"`
	EndOfLifeStart          string            `json:"endOfLifeStart"`
	ProductComingSoon       int               `json:"productComingSoon"`
	ProductNew              int               `json:"productNew"`
	ProductFocus            int               `json:"productFocus"`
	AlternativeProductSeries string           `json:"alternativeProductSeries"`
	AlternativeProduct      string            `json:"alternativeProduct"`
	Replaces                string            `json:"replaces"`
	Attributes              map[string]string `json:"attributes"`
	ID                      string            `json:"id,omitempty"`
}

// CurrentVariationItem represents a variation from the API
type CurrentVariationItem struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	LastChange string `json:"lastChange"`
}

// CurrentVariationStructure represents the API response for get-variations
type CurrentVariationStructure map[string]CurrentVariationItem

var (
	apiHost = "https://recom-power.cn/wp-json/recom-wordpress/v1/"
	
	// 统计变量
	addedCount   int // 新增变体计数
	updatedCount int // 更新变体计数
)

func main() {
	response, err := handler()
	if err != nil {
		fmt.Printf("Error occurred: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Function completed with status code: %d\n", response.StatusCode)
}

type Response struct {
	StatusCode int `json:"statusCode"`
}

func handler() (*Response, error) {
	// 初始化统计变量
	addedCount = 0
	updatedCount = 0
	
	fmt.Println("Starting variation synchronization...")

	// Initialize variables.
	catalogVariationStructure := make(map[string]*CatalogVariation)
	currentVariationStructure := make(CurrentVariationStructure)

	// Load the catalog into the memory.
	fmt.Println("Loading catalog file...")
	catalogData, err := ioutil.ReadFile("/usr/share/logstash/data/recom-catalog.xml")
	if err != nil {
		fmt.Printf("Failed to read products.xml: %v\n", err)
		return &Response{StatusCode: 500}, err
	}
	fmt.Printf("Catalog file size: %d characters\n", len(catalogData))

	// Parse the catalog.
	fmt.Println("Parsing XML catalog...")
	var catalog ProductSeries
	err = xml.Unmarshal(catalogData, &catalog)
	if err != nil {
		fmt.Printf("Failed to parse XML: %v\n", err)
		return &Response{StatusCode: 500}, err
	}
	fmt.Println("XML parsing completed")

	// Clear catalog data from memory
	catalogData = nil

	// Build the variation structure.
	fmt.Println("Building variation structure...")
	productSeriesList := catalog.ProductSeries
	fmt.Printf("Processing %d product series\n", len(productSeriesList))

	for i, product := range productSeriesList {
		if len(product.Products.Product) > 0 {
			for _, variation := range product.Products.Product {
				catalogVariationStructure[variation.PartNumber] = addVariationToCatalog(product, variation)
			}
		}

		// Log progress for large datasets
		if (i+1)%100 == 0 {
			fmt.Printf("Processed %d/%d product series\n", i+1, len(productSeriesList))
		}
	}

	fmt.Printf("Variation structure built: %d variations\n", len(catalogVariationStructure))

	// Get and build current variation structure.
	fmt.Println("Fetching current variations from API...")
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	
	resp, err := client.Get(apiHost + "get-variations")
	if err != nil {
		fmt.Printf("Failed to fetch current variations: %v\n", err)
		return &Response{StatusCode: 500}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &Response{StatusCode: 500}, err
	}

	if resp.StatusCode != http.StatusOK {
		return &Response{StatusCode: 500}, fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Check if response is an empty array
	if string(body) == "[]" {
		fmt.Println("No existing variations found.")
	} else {
		err = json.Unmarshal(body, &currentVariationStructure)
		if err != nil {
			return &Response{StatusCode: 500}, fmt.Errorf("failed to unmarshal response: %v", err)
		}
		fmt.Printf("Retrieved %d existing variations\n", len(currentVariationStructure))
	}

	// Synchronize variation with batch processing.
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("📊 Processing %d total variations\n", len(catalogVariationStructure))

	// 根据数据量调整批量参数
	totalVariations := len(catalogVariationStructure)
	batchSize := 50
	maxConcurrent := 5 // Reduce concurrency to avoid issues

	if totalVariations > 5000 {
		batchSize = 100  // 大数据集使用更大批次
		maxConcurrent = 3 // 减少并发避免服务器过载
	} else if totalVariations < 500 {
		batchSize = 25   // 小数据集使用小批次
		maxConcurrent = 10 // 增加并发提升速度
	}

	fmt.Printf("⚙️  Batch configuration: BATCH_SIZE=%d, MAX_CONCURRENT=%d\n", batchSize, maxConcurrent)

	var addVariations []*CatalogVariation
	var updateVariations []*CatalogVariation

	// 分类需要添加和更新的变体
	for _, variation := range catalogVariationStructure {
		if currentItem, exists := currentVariationStructure[variation.VariationCode]; exists {
			// Convert string lastChange from API to int64 for comparison
			currentLastChange, err := strconv.ParseInt(currentItem.LastChange, 10, 64)
			if err != nil {
				fmt.Printf("Warning: Failed to parse lastChange for existing variation %s: %v\n", variation.VariationCode, err)
				currentLastChange = 0
			}

			if currentLastChange != variation.LastChange {
				variation.ID = currentItem.ID
				updateVariations = append(updateVariations, variation)
			}
		} else {
			addVariations = append(addVariations, variation)
		}
	}

	fmt.Println("📈 Analysis completed:")
	fmt.Printf("   - New variations to add: %d\n", len(addVariations))
	fmt.Printf("   - Existing variations to update: %d\n", len(updateVariations))
	fmt.Printf("   - Unchanged variations: %d\n", totalVariations-len(addVariations)-len(updateVariations))

	processingStartTime := time.Now()

	// 批量处理新增变体
	if len(addVariations) > 0 {
		fmt.Printf("\n🔄 Processing %d new variations in batches...\n", len(addVariations))
		err = processBatches(addVariations, batchSize, maxConcurrent, "add")
		if err != nil {
			fmt.Printf("Warning: Failed to process add batches: %v\n", err)
		}
	}

	// 批量处理更新变体
	if len(updateVariations) > 0 {
		fmt.Printf("\n🔄 Processing %d updated variations in batches...\n", len(updateVariations))
		err = processBatches(updateVariations, batchSize, maxConcurrent, "update")
		if err != nil {
			fmt.Printf("Warning: Failed to process update batches: %v\n", err)
		}
	}

	processingDuration := time.Since(processingStartTime)
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("🎉 All processing completed in %.2f seconds\n", processingDuration.Seconds())
	fmt.Println("📊 Final summary:")
	fmt.Printf("   - Total variations processed: %d\n", len(addVariations)+len(updateVariations))
	fmt.Printf("   - Processing rate: %.1f variations/second\n", float64(len(addVariations)+len(updateVariations))/processingDuration.Seconds())
	fmt.Println(strings.Repeat("=", 50))
	
	// 将统计结果写入日志文件
	logMessage := fmt.Sprintf("[%s] Variation synchronization completed. Added: %d variations, Updated: %d variations\n", 
		time.Now().Format("2006-01-02 15:04:05"), addedCount, updatedCount)
	
	err = writeLog(logMessage)
	if err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}

	// Return success response.
	return &Response{StatusCode: 200}, nil
}

func processBatches(variations []*CatalogVariation, batchSize, maxConcurrent int, operation string) error {
	// 将变体分成批次
	var batches [][]*CatalogVariation
	for i := 0; i < len(variations); i += batchSize {
		end := i + batchSize
		if end > len(variations) {
			end = len(variations)
		}
		batches = append(batches, variations[i:end])
	}

	fmt.Printf("Processing %d variations in %d batches (%s)\n", len(variations), len(batches), operation)
	fmt.Printf("Batch size: %d, Max concurrent: %d\n", batchSize, maxConcurrent)

	processedBatches := 0

	// Sequentially process batches to avoid concurrency issues
	for i, batch := range batches {
		fmt.Printf("Processing batch %d/%d\n", i+1, len(batches))
		
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		
		err := processBatch(ctx, batch, i+1, operation, len(batches))
		
		cancel() // Cancel the context
		
		if err != nil {
			fmt.Printf("Error processing batch %d: %v\n", i+1, err)
		}
		
		processedBatches++
		fmt.Printf("Progress: %d/%d batches completed (%.1f%%)\n", processedBatches, len(batches), float64(processedBatches)/float64(len(batches))*100)
	}

	fmt.Printf("✅ Completed processing all %d variations for %s\n", len(variations), operation)
	return nil
}

func processBatch(ctx context.Context, batch []*CatalogVariation, batchNumber int, operation string, totalBatches int) error {
	fmt.Printf("📦 Starting batch %d/%d (%d variations)\n", batchNumber, totalBatches, len(batch))
	startTime := time.Now()

	var err error
	if operation == "add" {
		err = addVariationsBatch(ctx, batch)
		// 增加新增计数
		if err == nil {
			addedCount += len(batch)
		}
	} else if operation == "update" {
		err = updateVariationsBatch(ctx, batch)
		// 增加更新计数
		if err == nil {
			updatedCount += len(batch)
		}
	}

	if err != nil {
		fmt.Printf("❌ Batch %d failed: %v\n", batchNumber, err)

		// 如果批量失败，尝试逐个处理
		fmt.Printf("🔄 Retrying batch %d with individual requests...\n", batchNumber)
		successCount := 0
		failureCount := 0

		for _, variation := range batch {
			var individualErr error
			if operation == "add" {
				individualErr = addVariation(variation)
				if individualErr == nil {
					successCount++
					addedCount++ // 单个成功时也增加计数
				} else {
					failureCount++
					addedCount-- // 回退批量计数
				}
			} else if operation == "update" {
				individualErr = updateVariation(variation.ID, variation)
				if individualErr == nil {
					successCount++
					updatedCount++ // 单个成功时也增加计数
				} else {
					failureCount++
					updatedCount-- // 回退批量计数
				}
			}

			if individualErr != nil {
				fmt.Printf("❌ Individual variation failed: %s - %v\n", variation.VariationCode, individualErr)
			}
		}

		fmt.Printf("🔄 Batch %d retry completed: %d success, %d failures\n", batchNumber, successCount, failureCount)
		return err
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ Completed batch %d in %dms\n", batchNumber, duration.Milliseconds())
	return nil
}

func addVariationsBatch(ctx context.Context, variations []*CatalogVariation) error {
	fmt.Printf("Adding batch of %d variations\n", len(variations))

	// Prepare payload
	payload := map[string]interface{}{
		"variations": variations,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", apiHost+"add-variations-batch", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 120 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Successfully added %d variations\n", len(variations))
	
	// Try to parse response for errors
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err == nil {
		if errors, ok := response["errors"]; ok {
			fmt.Printf("Some variations had errors: %v\n", errors)
		}
	}

	return nil
}

func updateVariationsBatch(ctx context.Context, variations []*CatalogVariation) error {
	fmt.Printf("Updating batch of %d variations\n", len(variations))

	// Prepare payload
	payload := map[string]interface{}{
		"variations": variations,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", apiHost+"update-variations-batch", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 120 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Successfully updated %d variations\n", len(variations))
	
	// Try to parse response for errors
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err == nil {
		if errors, ok := response["errors"]; ok {
			fmt.Printf("Some variations had errors: %v\n", errors)
		}
	}

	return nil
}

func addVariationToCatalog(product ProductBase, variation Variation) *CatalogVariation {
	productCode := product.Code
	variationCode := variation.PartNumber

	// Parse lastChange timestamp
	lastChangeTime, err := time.Parse(time.RFC3339, variation.LastChange)
	var lastChange int64
	if err != nil {
		fmt.Printf("Warning: Failed to parse lastChange for variation %s: %v\n", variationCode, err)
		lastChange = 0
	} else {
		lastChange = lastChangeTime.Unix()
	}

	imageUrl := variation.ImageUrl

	// Replaces
	replaces := ""
	replacesFirst := true

	if len(variation.Replaces) > 0 {
		for _, replace := range variation.Replaces {
			if replace.Text != "" {
				if !replacesFirst {
					replaces = replaces + ", "
				}
				replaces = replaces + replace.Text
				if replacesFirst {
					replacesFirst = false
				}
			}
		}
	}

	// Thumbnail URL
	thumbnailUrl := variation.ThumbnailUrl

	datasheetUrl := variation.DatasheetUrl
	stepFileUrl := variation.StepFileUrl

	// Parse boolean values
	nrnd := parseBoolToInt(variation.Nrnd)
	endOfLife := parseBoolToInt(variation.EndOfLife)
	productComingSoon := parseBoolToInt(variation.ComingSoon)
	productNew := parseBoolToInt(variation.New)
	productFocus := parseBoolToInt(variation.Focus)

	endOfLifeStart := variation.EndOfLifeStart
	nrndStart := variation.NrndStart
	alternativeProductSeries := variation.AlternativeProductSeries
	alternativeProduct := variation.AlternativeProduct

	// Build attributes.
	attributes := make(map[string]string)
	features := variation.Features

	// Process numeric features
	for _, featureItem := range features.NumericFeature {
		attrKey := maybeCutAttributeName(featureItem.ID)
		attributes[attrKey] = featureItem.Value
	}

	// Process enum features
	for _, featureItem := range features.EnumFeature {
		attrKey := maybeCutAttributeName(featureItem.ID)
		attributes[attrKey] = featureItem.Value
	}

	// Process range features
	for _, featureItem := range features.RangeFeature {
		// Save as range in one field
		rangeKey := maybeCutAttributeName("range_" + featureItem.ID)
		attributes[rangeKey] = "[" + featureItem.From + ", " + featureItem.To + "]"

		// Save as one value in two (another) fields
		rangeFromKey := maybeCutAttributeName("rangefrom_" + featureItem.ID)
		attributes[rangeFromKey] = featureItem.From

		rangeToKey := maybeCutAttributeName("rangeto_" + featureItem.ID)
		attributes[rangeToKey] = featureItem.To
	}

	// Process set features
	for _, featureItem := range features.SetFeature {
		attrKey := maybeCutAttributeName("set_" + featureItem.ID)

		// Convert to JSON string
		valuesJSON, _ := json.Marshal(featureItem.Value)
		valuesStr := string(valuesJSON)
		attributes[attrKey] = valuesStr
	}

	// Process boolean features
	for _, featureItem := range features.BooleanFeature {
		attrKey := maybeCutAttributeName(featureItem.ID)
		attributes[attrKey] = featureItem.Value
	}

	// Process day features
	for _, featureItem := range features.DayFeature {
		attrKey := maybeCutAttributeName(featureItem.ID)
		attributes[attrKey] = featureItem.Value
	}

	// Process string features
	for _, featureItem := range features.StringFeature {
		attrKey := maybeCutAttributeName(featureItem.ID)
		attributes[attrKey] = featureItem.Value
	}

	return &CatalogVariation{
		ProductCode:             productCode,
		VariationCode:           variationCode,
		LastChange:              lastChange,
		ImageUrl:                imageUrl,
		ThumbnailUrl:            thumbnailUrl,
		DatasheetUrl:            datasheetUrl,
		StepFileUrl:             stepFileUrl,
		Nrnd:                    nrnd,
		NrndStart:               nrndStart,
		EndOfLife:               endOfLife,
		EndOfLifeStart:          endOfLifeStart,
		ProductComingSoon:       productComingSoon,
		ProductNew:              productNew,
		ProductFocus:            productFocus,
		AlternativeProductSeries: alternativeProductSeries,
		AlternativeProduct:      alternativeProduct,
		Replaces:                replaces,
		Attributes:              attributes,
	}
}

func addVariation(variation *CatalogVariation) error {
	fmt.Printf("Adding variation: %s\n", variation.VariationCode)

	// Prepare payload
	payload := map[string]interface{}{
		"productCode":             variation.ProductCode,
		"variationCode":           variation.VariationCode,
		"lastChange":              variation.LastChange,
		"imageUrl":                variation.ImageUrl,
		"thumbnailUrl":            variation.ThumbnailUrl,
		"datasheetUrl":            variation.DatasheetUrl,
		"stepFileUrl":             variation.StepFileUrl,
		"nrnd":                    variation.Nrnd,
		"nrndStart":               variation.NrndStart,
		"endOfLife":               variation.EndOfLife,
		"endOfLifeStart":          variation.EndOfLifeStart,
		"productComingSoon":       variation.ProductComingSoon,
		"productNew":              variation.ProductNew,
		"productFocus":            variation.ProductFocus,
		"alternativeProductSeries": variation.AlternativeProductSeries,
		"alternativeProduct":      variation.AlternativeProduct,
		"replaces":                variation.Replaces,
		"attributes":              variation.Attributes,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequest("POST", apiHost+"add-variation", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Variation %s added successfully.\n", variation.VariationCode)
	return nil
}

func updateVariation(variationID string, variation *CatalogVariation) error {
	fmt.Printf("Updating variation ID: %s (%s)\n", variationID, variation.VariationCode)

	// Convert string ID to int for API call
	id, err := strconv.Atoi(variationID)
	if err != nil {
		return fmt.Errorf("invalid variation ID: %s", variationID)
	}

	// Prepare payload
	payload := map[string]interface{}{
		"id":                      id,
		"lastChange":              variation.LastChange,
		"imageUrl":                variation.ImageUrl,
		"thumbnailUrl":            variation.ThumbnailUrl,
		"datasheetUrl":            variation.DatasheetUrl,
		"stepFileUrl":             variation.StepFileUrl,
		"nrnd":                    variation.Nrnd,
		"nrndStart":               variation.NrndStart,
		"endOfLife":               variation.EndOfLife,
		"endOfLifeStart":          variation.EndOfLifeStart,
		"productComingSoon":       variation.ProductComingSoon,
		"productNew":              variation.ProductNew,
		"productFocus":            variation.ProductFocus,
		"alternativeProductSeries": variation.AlternativeProductSeries,
		"alternativeProduct":      variation.AlternativeProduct,
		"replaces":                variation.Replaces,
		"attributes":              variation.Attributes,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequest("POST", apiHost+"update-variation", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Variation ID %s updated successfully.\n", variationID)
	return nil
}

func maybeCutAttributeName(name string) string {
	if len(name) > 28 {
		return name[:28]
	}
	return name
}

func parseBoolToInt(value string) int {
	if value == "true" {
		return 1
	} else if value == "false" {
		return 0
	}

	// Try to parse as integer
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}

	return 0
}

// writeLog 将日志信息写入指定文件
func writeLog(message string) error {
	file, err := os.OpenFile("/usr/share/logstash/data/statistics.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(message)
	return err
}