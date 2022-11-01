package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nais/fasit/pkg/feature"
)

func main() {
	fs := http.FileServer(http.Dir("./ui"))

	mux := http.NewServeMux()
	mux.Handle("/", fs)
	mux.HandleFunc("/mapping", mapping)

	fmt.Println("Serving on port http://127.0.0.1:8083")
	http.ListenAndServe("127.0.0.1:8083", mux)
}

type data struct {
	Template string                `json:"template"`
	Values   feature.MappingValues `json:"values"`
}

func mapping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var d data
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeJSONErr(w, err)
		return
	}

	mp := feature.Mapping{
		"result": {
			Template: d.Template,
		},
	}

	out := make(map[string]any)
	if err := mp.Generate(d.Values.Kind, &d.Values, out); err != nil {
		writeJSONErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func writeJSONErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
}
