package etler

// import (
// 	"context"
// 	"encoding/json"
// 	"net/http"
// 	"time"

// 	"github.com/labstack/echo/v4"
// )

// // RESTAPI is an HTTP server that exposes the pipeline as a REST API.
// type RESTAPI struct {
// 	*echo.Echo
// 	pipeline *Pipeline
// }

// // NewRESTAPI creates a new RESTAPI.
// func NewRESTAPI(pipeline *Pipeline) *RESTAPI {
// 	// Initialize the Elastic APM middleware.
// 	//
// 	// NOTE: Set the following env vars:
// 	// - ELASTIC_APM_SECRET_TOKEN: secret token for the APM server
// 	// - ELASTIC_APM_SERVER_URL: URL of the APM server
// 	//
// 	// - ELASTIC_APM_ENVIRONMENT: environment name
// 	// - ELASTIC_APM_SERVICE_NAME: name of the service
// 	// - ELASTIC_APM_SERVICE_VERSION: version of the service
// 	//
// 	// - ELASTIC_APM_ACTIVE: whether or not to enable the APM agent (true, or false)
// 	//
// 	// - ELASTIC_APM_LOG_LEVEL: log level for the APM agent (error, or debug)
// 	// - ELASTIC_APM_LOG_FILE: log file for the APM agent (file, stdout, or stderr)
// 	// - ELASTIC_APM_CAPTURE_HEADERS: capture request and response headers (true, or false)
// 	// - ELASTIC_APM_CAPTURE_BODY: capture request and response bodies (off, errors, transactions, all)
// 	//
// 	// SEE: https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html
// 	apmMiddleware, err := apmecho.Middleware("my-service")
// 	if err != nil {
// 		// Handle error.
// 	}

// 	e := echo.New()

// 	// Add the Elastic APM middleware to the Echo instance.
// 	e.Use(apmMiddleware)

// 	api := &RESTAPI{e, pipeline}

// 	// Set up the routes for the API.
// 	api.POST("/process", api.processHandler)

// 	return api
// }

// // processHandler is an HTTP handler for the '/process' route. It reads a request
// // body containing a slice of values of any type and a boolean flag indicating
// // whether the stages of the pipeline should be run concurrently. It then runs
// // the pipeline and returns the processed data in the response.
// func (a *RESTAPI) processHandler(c echo.Context) error {
// 	// Read the request body.
// 	var request struct {
// 		Data        []interface{} `json:"data"`
// 		Concurrent  bool          `json:"concurrent"`
// 		ContentType string        `json:"contentType"`
// 	}
// 	err := c.Bind(&request)
// 	if err != nil {
// 		return err
// 	}

// 	// Unmarshal the data into a slice of the specified type.
// 	var data []interface{}
// 	err = json.Unmarshal(request.Data, &data)
// 	if err != nil {
// 		return err
// 	}

// 	// Set up a context with a timeout.
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	// Run the pipeline.
// 	out, err := a.pipeline.Run(ctx, request.Concurrent, data)
// 	if err != nil {
// 		return err
// 	}

// 	// Marshal the output into JSON.
// 	output, err := json.Marshal(out)
// 	if err != nil {
// 		return err
// 	}

// 	// Set the content type and write the response.
// 	// c.Response().Header().Set(echo.HeaderContentType, request.Content

// 	// Send the response.
// 	return c.JSONBlob(http.StatusOK, output)
// }
