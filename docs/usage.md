# Usage

## Against a smee.io channel

1. Create a channel at <https://smee.io/> and copy its URL.
2. Point the source at the channel and the target at your local service:

   ```bash
   ./sse-webhook-tunnel \
     --source https://smee.io/your-channel \
     --target http://localhost:9000/hook
   ```

3. Configure the upstream (for example a GitHub webhook) to deliver to the smee
   channel URL. Deliveries appear in the TUI as timestamped lines and are
   forwarded to `http://localhost:9000/hook`.

## Local end-to-end example

You can exercise the tunnel without any external service using two small local
servers: one that emits an SSE stream and one that receives forwarded requests.

Sink (receives forwarded POSTs), save as `sink.go` and run with `go run sink.go`:

```go
package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.Printf("received %s %s: %s", r.Method, r.URL, string(body))
		w.WriteHeader(http.StatusOK)
	})
	log.Println("sink listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}
```

Source (emits one SSE event every few seconds), save as `source.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		id := 0
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(3 * time.Second):
				id++
				fmt.Fprintf(w, "id: %d\nevent: message\ndata: {\"x-github-event\":\"push\",\"body\":{\"seq\":%d}}\n\n", id, id)
				flusher.Flush()
			}
		}
	})
	log.Println("source listening on :9100")
	log.Fatal(http.ListenAndServe(":9100", nil))
}
```

Run the sink and source, then start the tunnel:

```bash
./sse-webhook-tunnel --source http://localhost:9100/stream --target http://localhost:9000/hook
```

Events appear in the TUI every three seconds; the sink logs each forwarded POST.
Press `enter` to inspect a payload, `pgup`/`pgdn` to page, and `:q` then `enter`
to quit (`ctrl+c` force-quits).
