package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var allowedOrigins = []string{
	"http://localhost:3000",
	"http://127.0.0.1:3000",
	"http://localhost:5173",
}

// Middleware для проверки CORS
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем источник запроса
		origin := r.Header.Get("Origin")

		// Проверяем, разрешен ли источник
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// Если источник разрешен, добавляем CORS заголовки
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		// Для preflight запросов (OPTIONS)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Вызываем следующий обработчик
		next(w, r)
	}
}

type ParseStats struct {
	TotalLines      int
	SuccessLines    int
	ErrorLines      int
	ByLevel         map[string]int
	ByModule        map[string]int
	HasHTTPRequests bool
}

type ParseResult struct {
	Stats  ParseStats
	Logs   []TerraformLog
	Errors []ParseError
}

type ParseError struct {
	LineNumber int
	Line       string
	Error      error
}

type TerraformLog struct {
	Level          string
	Message        string
	Module         string
	Caller         string
	Timestamp      time.Time
	TfReqID        string
	TfRPC          string
	TfProtoVersion string
	TfProviderAddr string
	EntryType      string
	RawJSON        string
}

type LogParser struct {
	stats ParseStats
}

// Глобальная переменная для хранения последних результатов
var currentResult *ParseResult

func NewLogParser() *LogParser {
	return &LogParser{
		stats: ParseStats{
			ByLevel:  make(map[string]int),
			ByModule: make(map[string]int),
		},
	}
}

// ParseStream - парсинг потока логов (файл или stdin)
func (p *LogParser) ParseStream(reader io.Reader) ParseResult {
	result := ParseResult{}

	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		p.stats.TotalLines++

		if line == "" {
			continue
		}

		logEntry, err := p.parseLine(line)
		if err != nil {
			result.Errors = append(result.Errors, ParseError{
				LineNumber: lineNumber,
				Line:       line,
				Error:      err,
			})
			p.stats.ErrorLines++
			continue
		}

		result.Logs = append(result.Logs, logEntry)
		p.stats.SuccessLines++
		p.updateStats(logEntry)
	}

	result.Stats = p.stats
	return result
}

// ParseFile - парсинг конкретного файла
func (p *LogParser) ParseFile(filename string) (ParseResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ParseResult{}, fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer file.Close()

	return p.ParseStream(file), nil
}

// ParseFiles - парсинг нескольких файлов
func (p *LogParser) ParseFiles(filenames []string) (ParseResult, error) {
	var allResult ParseResult

	for _, filename := range filenames {
		result, err := p.ParseFile(filename)
		if err != nil {
			return ParseResult{}, fmt.Errorf("ошибка при обработке файла %s: %w", filename, err)
		}

		// Объединяем результаты
		allResult.Logs = append(allResult.Logs, result.Logs...)
		allResult.Errors = append(allResult.Errors, result.Errors...)
	}

	allResult.Stats = p.stats
	return allResult, nil
}

// parseLine - парсинг одной строки лога
func (p *LogParser) parseLine(line string) (TerraformLog, error) {
	var logEntry TerraformLog
	var rawData map[string]interface{}

	// Парсим JSON в сырую мапу для гибкости
	if err := json.Unmarshal([]byte(line), &rawData); err != nil {
		return logEntry, fmt.Errorf("invalid JSON: %w", err)
	}

	// Обрабатываем основные поля
	logEntry.Level = getString(rawData, "@level")
	logEntry.Message = getString(rawData, "@message")
	logEntry.Module = getString(rawData, "@module")
	logEntry.Caller = getString(rawData, "@caller")
	logEntry.TfReqID = getString(rawData, "tf_req_id")
	logEntry.TfRPC = getString(rawData, "tf_rpc")
	logEntry.TfProtoVersion = getString(rawData, "tf_proto_version")
	logEntry.TfProviderAddr = getString(rawData, "tf_provider_addr")

	// Парсим timestamp
	if tsStr := getString(rawData, "@timestamp"); tsStr != "" {
		if timestamp, err := time.Parse(time.RFC3339, tsStr); err == nil {
			logEntry.Timestamp = timestamp
		}
	}

	// Определяем тип записи
	logEntry.EntryType = p.classifyEntry(logEntry, rawData)

	// Сохраняем оригинальный JSON для ленивой загрузки
	logEntry.RawJSON = line

	return logEntry, nil
}

// classifyEntry - классификация типа записи
func (p *LogParser) classifyEntry(log TerraformLog, rawData map[string]interface{}) string {
	// HTTP запросы
	if log.TfReqID != "" {
		p.stats.HasHTTPRequests = true
		return "http_request"
	}

	// GRPC запросы
	if strings.Contains(log.Message, "GRPCProvider") || log.TfRPC != "" {
		return "grpc_request"
	}

	// Сообщения от провайдеров
	if log.Module != "" && strings.Contains(log.Module, "provider") {
		return "provider"
	}

	return "general"
}

// updateStats - обновление статистики
func (p *LogParser) updateStats(logEntry TerraformLog) {
	p.stats.ByLevel[logEntry.Level]++
	if logEntry.Module != "" {
		p.stats.ByModule[logEntry.Module]++
	}
}

// getString - вспомогательная функция для извлечения строк из map
func getString(data map[string]interface{}, key string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// Серверные функции
func startWebServer(port string) {
	http.HandleFunc("/", handleMain)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/api/logs", corsMiddleware(handleAPILogs))
	http.HandleFunc("/api/status", corsMiddleware(handleAPIStatus))
	http.HandleFunc("/api/clear", corsMiddleware(handleAPIClear))

	fmt.Printf("Сервер запущен на http://localhost:%s\n", port)
	fmt.Println("Веб-интерфейс: http://localhost:" + port)
	fmt.Println("API эндпоинты:")
	fmt.Println("   POST /api/logs    - отправить логи")
	fmt.Println("   GET  /api/status  - получить статистику")
	fmt.Println("   POST /api/clear   - очистить логи")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Обработчик главной страницы
func handleMain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Terraform Log Parser</title>
    <meta charset="utf-8">
    <style>
        .api-example { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        code { background: #eee; padding: 2px 5px; }
    </style>
</head>
<body>
    <h1> Terraform Log Parser</h1>
    
    <h3> Загрузите файл с логами:</h3>
    <form action="/upload" method="post" enctype="multipart/form-data">
        <input type="file" name="logfile" accept=".json,.log,.txt">
        <input type="submit" value="Анализировать">
    </form>
    
    <hr>
    
    <h3> API эндпоинты:</h3>
    
    <div class="api-example">
        <h4> POST /api/logs - отправить логи</h4>
        <p><strong>Формат:</strong> Текст, по одной JSON строке на запись</p>
        <p><strong>Пример:</strong></p>
        <code>
curl -X POST http://localhost:8080/api/logs \<br>
  -H "Content-Type: text/plain" \<br>
  -d '{"@level":"info","@message":"test","@timestamp":"2025-09-09T15:31:32.757289+03:00"}'<br>
        </code>
    </div>
    
    <div class="api-example">
        <h4> GET /api/status - получить статистику</h4>
        <p><strong>Пример:</strong></p>
        <code>curl http://localhost:8080/api/status</code>
    </div>
    
    <div class="api-example">
        <h4> POST /api/clear - очистить логи</h4>
        <p><strong>Пример:</strong></p>
        <code>curl -X POST http://localhost:8080/api/clear</code>
    </div>
    
    <hr>
    <h3> Командная строка:</h3>
    <code>go run main.go файл1.json файл2.json</code>
    <hr>
`)

	if currentResult != nil {
		displayWebResults(w, currentResult)
	}

	fmt.Fprintf(w, `</body></html>`)
}

// Обработчик загрузки файлов
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("logfile")
	if err != nil {
		fmt.Fprintf(w, "Ошибка загрузки файла: %v<br><a href='/'>Назад</a>", err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<a href="/">← Назад</a><hr>`)
	fmt.Fprintf(w, "<h2> Анализ файла: %s</h2>", header.Filename)

	parser := NewLogParser()
	result := parser.ParseStream(file)
	currentResult = &result

	displayWebResults(w, &result)
}
func filterLogs(logs []TerraformLog, levelFilter, sinceFilter, untilFilter, searchFilter, moduleFilter, limitStr string) []TerraformLog {
	if len(logs) == 0 {
		return logs
	}

	var filtered []TerraformLog

	// Отладочная информация
	if sinceFilter != "" {
		sinceTime, err := parseTimeFlexible(sinceFilter)
		if err != nil {
			fmt.Printf("Ошибка парсинга since фильтра '%s': %v\n", sinceFilter, err)
		} else {
			fmt.Printf("since фильтр '%s' -> %v\n", sinceFilter, sinceTime)
		}
	}

	if untilFilter != "" {
		untilTime, err := parseTimeFlexible(untilFilter)
		if err != nil {
			fmt.Printf("Ошибка парсинга until фильтра '%s': %v\n", untilFilter, err)
		} else {
			fmt.Printf("until фильтр '%s' -> %v\n", untilFilter, untilTime)
		}
	}

	for _, log := range logs {
		// Фильтр по уровню
		if levelFilter != "" && !strings.EqualFold(log.Level, levelFilter) {
			continue
		}
		// Фильтр по модулю (регистронезависимый)
		if moduleFilter != "" {
			if !strings.EqualFold(log.Module, moduleFilter) {
				continue
			}
		}

		// Фильтр по времени (с)
		if sinceFilter != "" {
			sinceTime, err := parseTimeFlexible(sinceFilter)
			if err == nil && log.Timestamp.Before(sinceTime) {
				continue
			}
		}

		// Фильтр по времени (по)
		if untilFilter != "" {
			untilTime, err := parseTimeFlexible(untilFilter)
			if err == nil && log.Timestamp.After(untilTime) {
				continue
			}
		}

		// Поиск по сообщению (регистронезависимый)
		if searchFilter != "" {
			if !strings.Contains(strings.ToLower(log.Message), strings.ToLower(searchFilter)) {
				continue
			}
		}

		filtered = append(filtered, log)
	}

	// Применяем лимит
	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}

	fmt.Printf(" Фильтрация: из %d записей осталось %d\n", len(logs), len(filtered))
	return filtered
}

func parseTimeFlexible(timeStr string) (time.Time, error) {
	// Пробуем разные форматы
	formats := []string{
		time.RFC3339,          // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05", // без временной зоны
		"2006-01-02T15:04",    // с буквой T, без секунд - ДОБАВЬ ЭТОТ ФОРМАТ
		"2006-01-02 15:04:05", // с пробелом вместо T
		"2006-01-02 15:04",    // с пробелом, без секунд
		"2006-01-02",          // только дата
		"15:04:05",            // только время
		"15:04",               // только время без секунд
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("неверный формат времени: %s", timeStr)
}

// Функция для расчета статистики по отфильтрованным логам
func calculateFilteredStats(logs []TerraformLog) ParseStats {
	stats := ParseStats{
		ByLevel:  make(map[string]int),
		ByModule: make(map[string]int),
	}

	for _, log := range logs {
		stats.TotalLines++
		stats.SuccessLines++

		// Считаем статистику по уровням
		if log.Level != "" {
			stats.ByLevel[log.Level]++
		}

		// Считаем статистику по модулям
		if log.Module != "" {
			stats.ByModule[log.Module]++
		}

		// Проверяем наличие HTTP запросов
		if log.TfReqID != "" {
			stats.HasHTTPRequests = true
		}
	}

	return stats
}

// Обработчик API для приема логов
func handleAPILogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Обработка GET запроса - получение всех логов
	if r.Method == "GET" {
		if currentResult == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_data",
				"message": "Нет данных логов",
				"logs":    []interface{}{},
			})
			return
		}
		query := r.URL.Query()
		levelFilter := query.Get("level")   // Фильтр по уровню
		sinceFilter := query.Get("since")   // Фильтр по времени (с)
		untilFilter := query.Get("until")   // Фильтр по времени (по)
		searchFilter := query.Get("search") // Поиск по сообщению
		moduleFilter := query.Get("module") // Фильтр по модулю
		limitStr := query.Get("limit")      // Лимит записей

		// Фильтруем логи
		filteredLogs := filterLogs(currentResult.Logs, levelFilter, sinceFilter, untilFilter, searchFilter, moduleFilter, limitStr)

		// Рассчитываем статистику по отфильтрованным логам
		filteredStats := calculateFilteredStats(filteredLogs)

		response := map[string]interface{}{
			"status":         "success",
			"stats":          filteredStats,       // Используем отфильтрованную статистику
			"original_stats": currentResult.Stats, // Сохраняем оригинальную статистику для сравнения
			"filters": map[string]interface{}{
				"level":  levelFilter,
				"since":  sinceFilter,
				"until":  untilFilter,
				"search": searchFilter,
				"module": moduleFilter,
				"limit":  limitStr,
			},
			"logs":  filteredLogs,
			"count": len(filteredLogs),
			"total": len(currentResult.Logs),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Обработка DELETE запроса
	if r.Method == "DELETE" {
		// Очищаем все логи
		currentResult = nil

		response := map[string]interface{}{
			"status":  "success",
			"message": "Все логи успешно очищены",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var body []byte
	var err error

	// Проверяем Content-Type
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Обработка загрузки файла через форму
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error": "Ошибка чтения файла"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		body, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, `{"error": "Ошибка чтения содержимого файла"}`, http.StatusBadRequest)
			return
		}

		fmt.Printf("Получен файл: %s\n", header.Filename)
	} else {
		// Обработка обычного текста/JSON
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error": "Ошибка чтения тела запроса"}`, http.StatusBadRequest)
			return
		}
	}

	// Дальше твой существующий код для парсинга...
	parser := NewLogParser()

	// Если это JSON массив, парсим как массив
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err == nil {
		if array, ok := jsonData.([]interface{}); ok {
			// Это массив JSON объектов - объединяем в строки
			var lines []string
			for _, item := range array {
				if jsonBytes, err := json.Marshal(item); err == nil {
					lines = append(lines, string(jsonBytes))
				}
			}
			body = []byte(strings.Join(lines, "\n"))
		}
	}

	result := parser.ParseStream(strings.NewReader(string(body)))

	// Обновляем текущий результат...
	if currentResult == nil {
		currentResult = &result
	} else {
		currentResult.Logs = append(currentResult.Logs, result.Logs...)
		currentResult.Errors = append(currentResult.Errors, result.Errors...)
		currentResult.Stats.TotalLines += result.Stats.TotalLines
		currentResult.Stats.SuccessLines += result.Stats.SuccessLines
		currentResult.Stats.ErrorLines += result.Stats.ErrorLines

		for level, count := range result.Stats.ByLevel {
			currentResult.Stats.ByLevel[level] += count
		}
		for module, count := range result.Stats.ByModule {
			currentResult.Stats.ByModule[module] += count
		}
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Логи успешно обработаны",
		"added":   len(result.Logs),
		"errors":  len(result.Errors),
		"total":   len(currentResult.Logs),
	}

	json.NewEncoder(w).Encode(response)
}

// Обработчик API для получения статуса
func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if currentResult == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_data",
			"message": "Нет данных логов",
		})
		return
	}

	response := map[string]interface{}{
		"status":       "success",
		"stats":        currentResult.Stats,
		"logs_count":   len(currentResult.Logs),
		"errors_count": len(currentResult.Errors),
	}

	json.NewEncoder(w).Encode(response)
}

// Обработчик API для очистки логов
func handleAPIClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	currentResult = nil

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Все логи очищены",
	})
}

// Отображение результатов в веб-интерфейсе
func displayWebResults(w http.ResponseWriter, result *ParseResult) {
	// Статистика
	fmt.Fprintf(w, "<h3> Статистика:</h3>")
	fmt.Fprintf(w, "<pre>")
	fmt.Fprintf(w, "Всего строк: %d\n", result.Stats.TotalLines)
	fmt.Fprintf(w, "Успешно: %d\n", result.Stats.SuccessLines)
	fmt.Fprintf(w, "Ошибок: %d\n", result.Stats.ErrorLines)
	fmt.Fprintf(w, "\nПо уровням:\n")
	for level, count := range result.Stats.ByLevel {
		fmt.Fprintf(w, "  %s: %d\n", level, count)
	}
	fmt.Fprintf(w, "\n По модулям:\n")
	for module, count := range result.Stats.ByModule {
		fmt.Fprintf(w, "  %s: %d\n", module, count)
	}
	fmt.Fprintf(w, "</pre>")

	// Логи
	fmt.Fprintf(w, "<h3> Логи (%d записей):</h3>", len(result.Logs))

	// Показываем последние 100 записей
	logsToShow := result.Logs
	if len(logsToShow) > 100 {
		logsToShow = logsToShow[len(logsToShow)-100:]
		fmt.Fprintf(w, "<p><i>Показаны последние 100 записей из %d</i></p>", len(result.Logs))
	}

	for i, logEntry := range logsToShow {
		levelColor := "black"
		switch strings.ToLower(logEntry.Level) {
		case "error":
			levelColor = "red"
		case "warn", "warning":
			levelColor = "orange"
		case "info":
			levelColor = "green"
		case "debug":
			levelColor = "blue"
		case "trace":
			levelColor = "gray"
		}

		fmt.Fprintf(w, `
		<div style="border:1px solid #ddd; margin:5px 0; padding:10px; font-family: monospace;">
			<div><b>#%d</b> | <span style="color:%s">%s</span> | %s | <small>%s</small></div>
			<div><b>Сообщение:</b> %s</div>
			<div style="font-size:12px; color:#666;">
				%s | %s | %s
			</div>
		</div>
		`,
			i+1,
			levelColor, logEntry.Level,
			logEntry.Timestamp.Format("15:04:05"),
			logEntry.EntryType,
			logEntry.Message,
			logEntry.Module,
			logEntry.Caller,
			logEntry.TfReqID,
		)
	}

	// Ошибки парсинга
	if len(result.Errors) > 0 {
		fmt.Fprintf(w, "<h3>Ошибки парсинга (%d):</h3>", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Fprintf(w, `
			<div style="background:#ffe6e6; border:1px solid red; margin:2px; padding:5px;">
				<strong>Строка %d:</strong> %v<br>
				<small>%s</small>
			</div>
			`, err.LineNumber, err.Error, err.Line)
		}
	}
}

// printResults - вывод результатов парсинга (консольная версия)
func printResults(result ParseResult) {
	fmt.Printf("Обработано строк: %d\n", result.Stats.SuccessLines)
	fmt.Printf("Ошибок: %d\n", len(result.Errors))

	// Вывод первых 10 записей
	maxDisplay := 10
	if len(result.Logs) < maxDisplay {
		maxDisplay = len(result.Logs)
	}

	for i := 0; i < maxDisplay; i++ {
		log := result.Logs[i]
		fmt.Printf("[%d] %s %s: %s (%s)\n",
			i+1,
			log.Timestamp.Format("15:04:05"),
			log.Level,
			log.Message,
			log.EntryType,
		)
	}

	if len(result.Logs) > maxDisplay {
		fmt.Printf("... и еще %d записей\n", len(result.Logs)-maxDisplay)
	}

	// Вывод статистики
	fmt.Printf("\n=== Статистика ===\n")
	fmt.Printf("Всего строк: %d\n", result.Stats.TotalLines)
	fmt.Printf("Успешно: %d\n", result.Stats.SuccessLines)
	fmt.Printf("Ошибок: %d\n", result.Stats.ErrorLines)

	fmt.Printf("\nПо уровням:\n")
	for level, count := range result.Stats.ByLevel {
		fmt.Printf("  %s: %d\n", level, count)
	}

	fmt.Printf("\nПо модулям:\n")
	for module, count := range result.Stats.ByModule {
		fmt.Printf("  %s: %d\n", module, count)
	}

	// Вывод ошибок, если есть
	if len(result.Errors) > 0 {
		fmt.Printf("\n=== Ошибки парсинга ===\n")
		for i, err := range result.Errors {
			if i >= 5 {
				fmt.Printf("... и еще %d ошибок\n", len(result.Errors)-5)
				break
			}
			fmt.Printf("Строка %d: %v\n", err.LineNumber, err.Error)
		}
	}
}

func main() {
	// Проверяем аргументы командной строки
	if len(os.Args) > 1 {
		// Чтение из файла(ов)
		parser := NewLogParser()

		if os.Args[1] == "-" {
			// Чтение из stdin
			fmt.Println("Чтение логов из stdin...")
			result := parser.ParseStream(os.Stdin)
			printResults(result)
			currentResult = &result
			fmt.Println("\nЗапуск веб-сервера...")
			startWebServer("8080")
		} else {
			// Чтение из файла(ов)
			filenames := os.Args[1:]
			fmt.Printf("Обработка файлов: %v\n", filenames)

			result, err := parser.ParseFiles(filenames)
			if err != nil {
				log.Fatalf("Ошибка: %v", err)
			}
			printResults(result)
			currentResult = &result
			fmt.Println("\nЗапуск веб-сервера...")
			startWebServer("8080")
		}
	} else {
		// Запуск только сервера
		fmt.Println("Запуск Terraform Log Parser Server...")
		startWebServer("8080")
	}
}
