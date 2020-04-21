package grader

import (
	"fmt"
	"net/http"
)

func callApi(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Calling api\n")
}

func HandleRequest() {
  http.HandleFunc("/", callApi)
  http.ListenAndServe(":11112", nil)
}
