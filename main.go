package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/lib/pq"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PageData struct {
	Title          string
	QueryResult    string
	CurrentQuery   string
	ConnectionInfo *ConnectionInfo
}

type ConnectionInfo struct {
	Host     string
	Port     string
	DBName   string
	Username string
	Password string
}

var (
	db     *sql.DB
	tmpl   *template.Template
	dbName string
	dbUser string
	dbPass string
)

func main() {
	// Parse templates
	var err error
	tmpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Error parsing templates:", err)
	}

	// Set up routes
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/connect", connectHandler)
	http.HandleFunc("/execute", executeQueryHandler)
	http.HandleFunc("/save", saveQueryHandler)
	http.HandleFunc("/queries", queriesHandler)
	http.HandleFunc("/lab-queries", labQueriesHandler)
	http.HandleFunc("/create-table", createTableHandler)
	http.HandleFunc("/delete-table", deleteTableHandler)
	http.HandleFunc("/tables", listTablesHandler)
	http.HandleFunc("/edit-table", editTableHandler)
	http.HandleFunc("/table-info", tableInfoHandler)
	http.HandleFunc("/export-table", exportTableHandler)
	http.HandleFunc("/export-results", exportResultsHandler)
	http.HandleFunc("/backup", backupHandler)
	http.HandleFunc("/restore", restoreHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "SQL Query Tool",
	}
	tmpl.ExecuteTemplate(w, "base.html", data)
}

func connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Handle connection attempt
		info := ConnectionInfo{
			Host:     r.FormValue("host"),
			Port:     r.FormValue("port"),
			DBName:   r.FormValue("dbname"),
			Username: r.FormValue("username"),
			Password: r.FormValue("password"),
		}
		dbName = info.DBName
		dbUser = info.Username
		dbPass = info.Password

		connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
			info.Host, info.Port, info.DBName, info.Username, info.Password)

		var err error
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Connection successful!"))
		return
	}
	tmpl.ExecuteTemplate(w, "connection_modal.html", nil)
}

func executeQueryHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Empty query", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get columns: %v", err), http.StatusInternalServerError)
		return
	}

	var htmlTable strings.Builder
	htmlTable.WriteString("<table class='result-table'><thead><tr>")

	for _, col := range columns {
		htmlTable.WriteString(fmt.Sprintf("<th>%s</th>", col))
	}
	htmlTable.WriteString("</tr></thead><tbody>")

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))

	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan row: %v", err), http.StatusInternalServerError)
			return
		}

		htmlTable.WriteString("<tr>")
		for i := range columns {
			val := values[i]
			var strVal string
			switch v := val.(type) {
			case nil:
				strVal = "NULL"
			case []byte:
				strVal = string(v)
			default:
				strVal = fmt.Sprintf("%v", v)
			}
			htmlTable.WriteString(fmt.Sprintf("<td>%s</td>", template.HTMLEscapeString(strVal)))
		}
		htmlTable.WriteString("</tr>")
	}

	htmlTable.WriteString("</tbody></table>")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlTable.String()))
}

type SaveQueryRequest struct {
	QueryName string `json:"queryName"`
	QueryText string `json:"queryText"`
}

func saveQueryHandler(w http.ResponseWriter, r *http.Request) {
	var req SaveQueryRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.QueryName == "" || req.QueryText == "" {
		http.Error(w, "Query name and text are required", http.StatusBadRequest)
		return
	}

	err = os.MkdirAll("saved_queries", 0755)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	safeName := filepath.Base(req.QueryName)
	safeName = strings.TrimSuffix(safeName, filepath.Ext(safeName)) + ".sql"
	filePath := filepath.Join("saved_queries", safeName)

	err = os.WriteFile(filePath, []byte(req.QueryText), 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save query: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Query saved successfully!"))
}

type QueryFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func allQueriesHandler(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method == "GET" {
		files, err := os.ReadDir(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read queries: %v", err), http.StatusInternalServerError)
			return
		}

		var queryList []string
		for _, file := range files {
			if filepath.Ext(file.Name()) == ".sql" {
				queryList = append(queryList, file.Name())
			}
		}

		json.NewEncoder(w).Encode(queryList)
		return
	}

	if r.Method == "DELETE" {
		queryName := r.URL.Query().Get("name")
		if queryName == "" {
			http.Error(w, "Query name is required", http.StatusBadRequest)
			return
		}

		safeName := filepath.Base(queryName)
		filePath := filepath.Join(path, safeName)

		err := os.Remove(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete query: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Query deleted successfully"))
		return
	}

	if r.Method == "POST" {
		queryName := r.URL.Query().Get("name")
		if queryName == "" {
			http.Error(w, "Query name is required", http.StatusBadRequest)
			return
		}

		safeName := filepath.Base(queryName)
		filePath := filepath.Join(path, safeName)

		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read query: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Println(string(content))

		json.NewEncoder(w).Encode(QueryFile{
			Name:    queryName,
			Content: string(content),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func queriesHandler(w http.ResponseWriter, r *http.Request) {
	allQueriesHandler(w, r, "saved_queries")
}

func labQueriesHandler(w http.ResponseWriter, r *http.Request) {
	allQueriesHandler(w, r, "saved_lab_queries")
}

type CreateTableRequest struct {
	TableName string        `json:"tableName"`
	Columns   []TableColumn `json:"columns"`
}

type TableColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func createTableHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateTableRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TableName == "" || len(req.Columns) == 0 {
		http.Error(w, "Table name and at least one column are required", http.StatusBadRequest)
		return
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("CREATE TABLE %s (", req.TableName))

	for i, col := range req.Columns {
		if i > 0 {
			queryBuilder.WriteString(", ")
		}
		queryBuilder.WriteString(fmt.Sprintf("%s %s", col.Name, col.Type))
	}
	queryBuilder.WriteString(")")

	_, err = db.Exec(queryBuilder.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create table: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Table %s created successfully!", req.TableName)))
}

type TableRequest struct {
	TableName string `json:"tableName"`
}

func listTablesHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query tables: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan table name: %v", err), http.StatusInternalServerError)
			return
		}
		tables = append(tables, table)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func deleteTableHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		var req TableRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TableName == "" {
			http.Error(w, "Table name is required", http.StatusBadRequest)
			return
		}

		_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", req.TableName))
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete table: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(fmt.Sprintf("Table %s deleted successfully", req.TableName)))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type TableInfoResponse struct {
	TableName string        `json:"tableName"`
	Columns   []TableColumn `json:"columns"`
}

type EditTableRequest struct {
	TableName string        `json:"tableName"`
	Columns   []TableColumn `json:"columns"`
}

func tableInfoHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	tableName := r.URL.Query().Get("name")
	if tableName == "" {
		http.Error(w, "Table name is required", http.StatusBadRequest)
		return
	}

	query := `
        SELECT column_name, data_type 
        FROM information_schema.columns 
        WHERE table_name = $1 AND table_schema = 'public'
        ORDER BY ordinal_position
    `
	rows, err := db.Query(query, tableName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table columns: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []TableColumn
	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan column info: %v", err), http.StatusInternalServerError)
			return
		}
		columns = append(columns, TableColumn{Name: name, Type: dataType})
	}

	response := TableInfoResponse{
		TableName: tableName,
		Columns:   columns,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func editTableHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	var req EditTableRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TableName == "" || len(req.Columns) == 0 {
		http.Error(w, "Table name and at least one column are required", http.StatusBadRequest)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to begin transaction: %v", err), http.StatusInternalServerError)
		return
	}

	rows, err := tx.Query(fmt.Sprintf("SELECT * FROM %s", req.TableName))
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to query existing data: %v", err), http.StatusInternalServerError)
		return
	}

	oldColumns, err := rows.Columns()
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to get columns: %v", err), http.StatusInternalServerError)
		return
	}

	var data [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(oldColumns))
		valuePtrs := make([]interface{}, len(oldColumns))
		for i := range oldColumns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to scan row: %v", err), http.StatusInternalServerError)
			return
		}
		data = append(data, values)
	}
	rows.Close()

	_, err = tx.Exec(fmt.Sprintf("DROP TABLE %s", req.TableName))
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to drop table: %v", err), http.StatusInternalServerError)
		return
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("CREATE TABLE %s (", req.TableName))

	for i, col := range req.Columns {
		if i > 0 {
			queryBuilder.WriteString(", ")
		}
		queryBuilder.WriteString(fmt.Sprintf("%s %s", col.Name, col.Type))
	}
	queryBuilder.WriteString(")")

	_, err = tx.Exec(queryBuilder.String())
	if err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to create table: %v", err), http.StatusInternalServerError)
		return
	}

	if len(data) > 0 {
		var insertBuilder strings.Builder
		insertBuilder.WriteString(fmt.Sprintf("INSERT INTO %s (", req.TableName))

		for i, col := range req.Columns {
			if i > 0 {
				insertBuilder.WriteString(", ")
			}
			insertBuilder.WriteString(col.Name)
		}
		insertBuilder.WriteString(") VALUES (")

		for i := range req.Columns {
			if i > 0 {
				insertBuilder.WriteString(", ")
			}
			insertBuilder.WriteString(fmt.Sprintf("$%d", i+1))
		}
		insertBuilder.WriteString(")")

		stmt, err := tx.Prepare(insertBuilder.String())
		if err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to prepare insert statement: %v", err), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		for _, row := range data {
			_, err := stmt.Exec(row...)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Failed to insert data: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit transaction: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Table %s updated successfully!", req.TableName)))
}

func exportTableHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	tableName := r.FormValue("tableName")
	if tableName == "" {
		http.Error(w, "Table name is required", http.StatusBadRequest)
		return
	}

	err := os.MkdirAll("saved_csv_tables", 0755)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	safeName := filepath.Base(tableName) + ".csv"
	filePath := filepath.Join("saved_csv_tables", safeName)

	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create CSV file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get columns: %v", err), http.StatusInternalServerError)
		return
	}
	if err := writer.Write(columns); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write CSV headers: %v", err), http.StatusInternalServerError)
		return
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan row: %v", err), http.StatusInternalServerError)
			return
		}

		record := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				record[i] = "NULL"
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := writer.Write(record); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write CSV row: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Error during rows iteration: %v", err), http.StatusInternalServerError)
		return
	}

	absPath, _ := filepath.Abs(filePath)
	w.Write([]byte(absPath))
}

func exportResultsHandler(w http.ResponseWriter, r *http.Request) {
	htmlTable := r.FormValue("htmlTable")
	if htmlTable == "" {
		http.Error(w, "No results to export", http.StatusBadRequest)
		return
	}

	err := os.MkdirAll("saved_csv_results", 0755)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	safeName := fmt.Sprintf("query_result_%s.csv", timestamp)
	filePath := filepath.Join("saved_csv_results", safeName)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlTable))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse HTML: %v", err), http.StatusInternalServerError)
		return
	}
	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create CSV file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	var headers []string
	doc.Find("thead th").Each(func(i int, s *goquery.Selection) {
		headers = append(headers, s.Text())
	})
	if err := writer.Write(headers); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write CSV headers: %v", err), http.StatusInternalServerError)
		return
	}

	// Write data rows
	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		var row []string
		s.Find("td").Each(func(j int, td *goquery.Selection) {
			row = append(row, td.Text())
		})
		if err := writer.Write(row); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write CSV row: %v", err), http.StatusInternalServerError)
			return
		}
	})

	absPath, _ := filepath.Abs(filePath)
	w.Write([]byte(absPath))
}

func backupDB(db *sql.DB, host, port, dbname, user, password, outputPath string) error {
	cmd := exec.Command(
		"pg_dump",
		"-h", host,
		"-p", port,
		"-U", user,
		"-d", dbname,
		"-f", outputPath,
		"-F", "c",
		"-v",
	)

	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %v\n%s", err, stderr.String())
	}
	return nil
}

type BackupResponse struct {
	FilePath string `json:"filePath"`
}

func backupHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	backupDir := "backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create backup directory: %v", err), http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFile := fmt.Sprintf("%s/backup_%s.sql", backupDir, timestamp)

	file, err := os.Create(backupFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create backup file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	file.WriteString("-- Database Backup\n")
	file.WriteString("-- Generated at: " + time.Now().Format(time.RFC1123) + "\n\n")

	err = backupDB(db, "localhost", "5432", dbName, dbUser, dbPass, backupFile)
	if err != nil {
		log.Fatal(err)
	}
	response := BackupResponse{
		FilePath: backupFile,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func restoreHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Not connected to database", http.StatusBadRequest)
		return
	}

	if r.Method == "GET" {
		// List available backup files
		files, err := os.ReadDir("backups")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read backups directory: %v", err), http.StatusInternalServerError)
			return
		}

		var backupFiles []string
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".sql") || strings.HasSuffix(file.Name(), ".dump") {
				backupFiles = append(backupFiles, file.Name())
			}
		}

		json.NewEncoder(w).Encode(backupFiles)
		return
	}

	if r.Method == "POST" {
		// Execute restore
		backupFile := r.FormValue("backupFile")
		if backupFile == "" {
			http.Error(w, "Backup file is required", http.StatusBadRequest)
			return
		}

		// Validate file exists
		filePath := filepath.Join("backups", backupFile)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "Backup file does not exist", http.StatusBadRequest)
			return
		}

		// Execute pg_restore
		cmd := exec.Command(
			"pg_restore",
			"-h", "localhost",
			"-p", "5432",
			"-U", dbUser,
			"-d", dbName,
			"-v",
			"--clean",
			"--create",
			filePath,
		)

		cmd.Env = append(os.Environ(), "PGPASSWORD="+dbPass)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		cmd.Run()

		w.Write([]byte("Database restored successfully"))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
