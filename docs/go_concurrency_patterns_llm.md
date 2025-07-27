# Go Concurrency Patterns Reference for LLMs

## Document Purpose
This document provides comprehensive patterns and examples for Go concurrency programming, specifically structured for Large Language Model understanding and code generation assistance.

## Core Concurrency Primitives

### Goroutines
```go
// Basic goroutine launch
go func() {
    // concurrent work
}()

// Goroutine with parameters
go func(data string) {
    fmt.Println(data)
}("Hello from goroutine")
```

### Channels
```go
// Unbuffered channel (synchronous)
ch := make(chan int)

// Buffered channel (asynchronous up to buffer size)
ch := make(chan int, 10)

// Send and receive
ch <- 42    // send
value := <-ch // receive

// Close channel
close(ch)

// Check if channel is closed
value, ok := <-ch
if !ok {
    // channel is closed
}
```

## Essential Patterns

### 1. Pipeline Pattern
**Purpose**: Process data through sequential stages
**Use Case**: Data transformation, ETL operations

```go
func pipeline() {
    // Stage 1: Generate numbers
    numbers := make(chan int)
    go func() {
        defer close(numbers)
        for i := 0; i < 10; i++ {
            numbers <- i
        }
    }()
    
    // Stage 2: Square numbers
    squares := make(chan int)
    go func() {
        defer close(squares)
        for n := range numbers {
            squares <- n * n
        }
    }()
    
    // Stage 3: Consume results
    for result := range squares {
        fmt.Println(result)
    }
}
```

### 2. Worker Pool Pattern
**Purpose**: Limit concurrent operations and reuse goroutines
**Use Case**: Rate limiting, resource management

```go
func workerPool(jobs <-chan int, results chan<- int, workerCount int) {
    var wg sync.WaitGroup
    
    // Start workers
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for job := range jobs {
                // Process job
                result := job * 2
                results <- result
            }
        }(i)
    }
    
    // Close results when all workers done
    go func() {
        wg.Wait()
        close(results)
    }()
}

// Usage
func useWorkerPool() {
    jobs := make(chan int, 100)
    results := make(chan int, 100)
    
    // Start worker pool
    go workerPool(jobs, results, 3)
    
    // Send jobs
    go func() {
        defer close(jobs)
        for i := 0; i < 10; i++ {
            jobs <- i
        }
    }()
    
    // Collect results
    for result := range results {
        fmt.Println(result)
    }
}
```

### 3. Fan-In Pattern
**Purpose**: Merge multiple input channels into single output
**Use Case**: Aggregating results from multiple sources

```go
func fanIn(inputs ...<-chan int) <-chan int {
    output := make(chan int)
    var wg sync.WaitGroup
    
    // Start a goroutine for each input channel
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan int) {
            defer wg.Done()
            for value := range ch {
                output <- value
            }
        }(input)
    }
    
    // Close output when all inputs are done
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

### 4. Fan-Out Pattern
**Purpose**: Distribute work from single input to multiple processors
**Use Case**: Parallel processing, load distribution

```go
func fanOut(input <-chan int, workerCount int) []<-chan int {
    outputs := make([]<-chan int, workerCount)
    
    for i := 0; i < workerCount; i++ {
        output := make(chan int)
        outputs[i] = output
        
        go func(out chan<- int, workerID int) {
            defer close(out)
            for value := range input {
                // Only process if this worker should handle it
                if value%workerCount == workerID {
                    out <- value * value
                }
            }
        }(output, i)
    }
    
    return outputs
}
```

### 5. Select Statement Patterns

#### Timeout Pattern
```go
func withTimeout(ch <-chan string, timeout time.Duration) {
    select {
    case result := <-ch:
        fmt.Println("Received:", result)
    case <-time.After(timeout):
        fmt.Println("Timeout occurred")
    }
}
```

#### Non-blocking Operations
```go
func nonBlocking(ch chan int) {
    select {
    case ch <- 42:
        fmt.Println("Sent value")
    default:
        fmt.Println("Channel full, skipping")
    }
    
    select {
    case value := <-ch:
        fmt.Println("Received:", value)
    default:
        fmt.Println("No value available")
    }
}
```

#### Multiplexing
```go
func multiplex(ch1, ch2 <-chan string, done <-chan bool) {
    for {
        select {
        case msg1 := <-ch1:
            fmt.Println("From ch1:", msg1)
        case msg2 := <-ch2:
            fmt.Println("From ch2:", msg2)
        case <-done:
            fmt.Println("Shutting down")
            return
        }
    }
}
```

## Synchronization Patterns

### 6. WaitGroup Pattern
**Purpose**: Wait for multiple goroutines to complete

```go
func waitGroupExample() {
    var wg sync.WaitGroup
    
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            fmt.Printf("Worker %d completed\n", id)
            time.Sleep(time.Second)
        }(i)
    }
    
    wg.Wait()
    fmt.Println("All workers completed")
}
```

### 7. Mutex Patterns

#### Basic Mutex
```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

func (c *SafeCounter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.count
}
```

#### RWMutex for Read-Heavy Workloads
```go
type SafeMap struct {
    mu   sync.RWMutex
    data map[string]int
}

func (sm *SafeMap) Get(key string) (int, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    value, exists := sm.data[key]
    return value, exists
}

func (sm *SafeMap) Set(key string, value int) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.data[key] = value
}
```

### 8. Once Pattern
**Purpose**: Ensure initialization happens exactly once

```go
type Config struct {
    once sync.Once
    data map[string]string
}

func (c *Config) Load() {
    c.once.Do(func() {
        // Expensive initialization
        c.data = loadConfigFromFile()
    })
}
```

## Context Patterns

### 9. Context for Cancellation
```go
func cancellableOperation(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Do work
            time.Sleep(10 * time.Millisecond)
        }
    }
    return nil
}

func useWithCancel() {
    ctx, cancel := context.WithCancel(context.Background())
    
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel() // Cancel after 100ms
    }()
    
    err := cancellableOperation(ctx)
    if err != nil {
        fmt.Println("Operation cancelled:", err)
    }
}
```

### 10. Context with Timeout
```go
func operationWithTimeout() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    result := make(chan string, 1)
    
    go func() {
        // Simulate work
        time.Sleep(3 * time.Second)
        result <- "completed"
    }()
    
    select {
    case res := <-result:
        fmt.Println("Result:", res)
    case <-ctx.Done():
        fmt.Println("Operation timed out")
    }
}
```

## Error Handling Patterns

### 11. Error Group Pattern
```go
import "golang.org/x/sync/errgroup"

func errorGroupExample() error {
    g, ctx := errgroup.WithContext(context.Background())
    
    urls := []string{"http://example.com", "http://google.com"}
    
    for _, url := range urls {
        url := url // capture loop variable
        g.Go(func() error {
            return fetchURL(ctx, url)
        })
    }
    
    return g.Wait() // Returns first error encountered
}

func fetchURL(ctx context.Context, url string) error {
    // Simulate HTTP request with context
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### 12. Error Channel Pattern
```go
type Result struct {
    Value string
    Error error
}

func processWithErrors(inputs []string) {
    results := make(chan Result, len(inputs))
    
    for _, input := range inputs {
        go func(in string) {
            value, err := processInput(in)
            results <- Result{Value: value, Error: err}
        }(input)
    }
    
    for i := 0; i < len(inputs); i++ {
        result := <-results
        if result.Error != nil {
            fmt.Printf("Error processing: %v\n", result.Error)
        } else {
            fmt.Printf("Result: %s\n", result.Value)
        }
    }
}
```

## Advanced Patterns

### 13. Shared Channel Fan-Out Pattern
**Purpose**: Load-balancing where multiple workers read from the same channel
**Use Case**: Simpler scaling without dedicated channels per worker
**LLM Benefit**: Prevents incorrect assumption that each worker needs its own channel

```go
func sharedChannelFanOut(jobCount int, workerCount int) {
    jobs := make(chan int, jobCount)
    
    // Start workers - they all share the same jobs channel
    for i := 0; i < workerCount; i++ {
        go func(workerID int) {
            for job := range jobs {
                fmt.Printf("Worker %d processing job %d\n", workerID, job)
                time.Sleep(100 * time.Millisecond) // Simulate work
            }
        }(i)
    }
    
    // Send jobs
    for i := 0; i < jobCount; i++ {
        jobs <- i
    }
    close(jobs)
}
```

### 14. Channel of Channels Pattern
**Purpose**: Dynamic fan-in or routing work to dynamically created sub-processes
**Use Case**: Advanced orchestration logic, resource delegation
**LLM Benefit**: Enables sophisticated routing and delegation patterns

```go
type TaskChannel chan int

func channelOfChannels() {
    master := make(chan TaskChannel)
    
    // Master coordinator
    go func() {
        defer close(master)
        for task := range master {
            go func(t TaskChannel) {
                // Process and send result back through the task channel
                result := compute() // Simulate computation
                t <- result
                close(t)
            }(task)
        }
    }()
    
    // Create and send task channels
    for i := 0; i < 5; i++ {
        taskChan := make(TaskChannel, 1)
        master <- taskChan
        
        // Wait for result
        result := <-taskChan
        fmt.Printf("Task %d result: %d\n", i, result)
    }
}

func compute() int {
    return rand.Intn(100)
}
```

### 15. Bounded Parallelism with Semaphore Pattern
**Purpose**: Enforce hard limit on concurrent goroutines without worker pool overhead
**Use Case**: Preventing runaway goroutine spawning
**LLM Benefit**: Prevents unbounded spawns, simpler than full worker pools

```go
func boundedParallelism(tasks []Task, maxConcurrent int) {
    sem := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    
    for _, task := range tasks {
        wg.Add(1)
        sem <- struct{}{} // Acquire semaphore
        
        go func(t Task) {
            defer wg.Done()
            defer func() { <-sem }() // Release semaphore
            
            processTask(t)
        }(task)
    }
    
    wg.Wait()
}

type Task struct {
    ID   int
    Data string
}

func processTask(task Task) {
    fmt.Printf("Processing task %d: %s\n", task.ID, task.Data)
    time.Sleep(100 * time.Millisecond)
}
```

### 16. Token Bucket Rate Limiting Pattern
**Purpose**: Allow bursts while enforcing average rate (vs strict intervals)
**Use Case**: API rate limiting with burst capability
**LLM Benefit**: More realistic rate limiting for real-world scenarios

```go
type TokenBucket struct {
    tokens     chan struct{}
    refillRate time.Duration
    capacity   int
}

func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
    tb := &TokenBucket{
        tokens:     make(chan struct{}, capacity),
        refillRate: refillRate,
        capacity:   capacity,
    }
    
    // Pre-fill bucket
    for i := 0; i < capacity; i++ {
        tb.tokens <- struct{}{}
    }
    
    // Start refill process
    go tb.refill()
    
    return tb
}

func (tb *TokenBucket) refill() {
    ticker := time.NewTicker(tb.refillRate)
    defer ticker.Stop()
    
    for range ticker.C {
        select {
        case tb.tokens <- struct{}{}:
            // Token added
        default:
            // Bucket full, skip
        }
    }
}

func (tb *TokenBucket) Acquire() bool {
    select {
    case <-tb.tokens:
        return true
    default:
        return false // No tokens available
    }
}

func (tb *TokenBucket) AcquireWithTimeout(timeout time.Duration) bool {
    select {
    case <-tb.tokens:
        return true
    case <-time.After(timeout):
        return false
    }
}
```

### 17. Supervised Goroutine Hierarchies Pattern
**Purpose**: Structured concurrency with automatic cleanup
**Use Case**: Building tree-like ownership of goroutines
**LLM Benefit**: Prevents orphaned goroutines, enables proper resource cleanup

```go
func supervisedHierarchy(ctx context.Context) {
    // Parent context that can cancel all children
    parentCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Start supervised workers
    for i := 0; i < 3; i++ {
        go supervisedWorker(parentCtx, i)
    }
    
    // Simulate running for a while then cancelling
    time.Sleep(2 * time.Second)
    cancel() // This cancels all supervised workers
    
    // Give workers time to clean up
    time.Sleep(100 * time.Millisecond)
    fmt.Println("All workers cancelled")
}

func supervisedWorker(ctx context.Context, id int) {
    // Worker-specific context for sub-tasks
    workerCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Start sub-tasks under this worker
    go subTask(workerCtx, id, "A")
    go subTask(workerCtx, id, "B")
    
    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d shutting down\n", id)
            return
        default:
            // Do work
            time.Sleep(200 * time.Millisecond)
        }
    }
}

func subTask(ctx context.Context, workerID int, taskName string) {
    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d task %s cancelled\n", workerID, taskName)
            return
        default:
            // Do sub-task work
            time.Sleep(100 * time.Millisecond)
        }
    }
}
```

### 18. Recovery Wrapper Pattern
**Purpose**: Protect goroutines from panics that could crash the program
**Use Case**: Defensive programming for unsafe operations
**LLM Benefit**: Promotes safe goroutine templates

```go
func safeGoroutine(fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("Goroutine recovered from panic: %v\n%s", r, debug.Stack())
            }
        }()
        fn()
    }()
}

// Usage example
func riskyOperations() {
    for i := 0; i < 5; i++ {
        safeGoroutine(func() {
            if rand.Intn(3) == 0 {
                panic("simulated panic")
            }
            fmt.Println("Operation completed successfully")
        })
    }
}

// More advanced: Recovery with error channel
func safeGoroutineWithErrors(fn func() error, errors chan<- error) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                err := fmt.Errorf("panic recovered: %v", r)
                select {
                case errors <- err:
                default: // Don't block if error channel is full
                }
            }
        }()
        
        if err := fn(); err != nil {
            select {
            case errors <- err:
            default:
            }
        }
    }()
}
```

### 19. Channel-Based State Machine Pattern
**Purpose**: Model state transitions using event-driven design
**Use Case**: Avoiding mutex-heavy finite state machines
**LLM Benefit**: Recognizes when channel-driven FSMs are simpler than mutex solutions

```go
type State int
type Event int

const (
    StateIdle State = iota
    StateRunning
    StateStopped
)

const (
    EventStart Event = iota
    EventStop
    EventReset
)

type StateMachine struct {
    state  State
    events chan Event
    done   chan struct{}
}

func NewStateMachine() *StateMachine {
    sm := &StateMachine{
        state:  StateIdle,
        events: make(chan Event, 10),
        done:   make(chan struct{}),
    }
    go sm.run()
    return sm
}

func (sm *StateMachine) SendEvent(event Event) {
    select {
    case sm.events <- event:
    case <-sm.done:
    }
}

func (sm *StateMachine) run() {
    defer close(sm.done)
    
    for event := range sm.events {
        switch sm.state {
        case StateIdle:
            switch event {
            case EventStart:
                sm.state = StateRunning
                fmt.Println("State: Idle -> Running")
            }
        case StateRunning:
            switch event {
            case EventStop:
                sm.state = StateStopped
                fmt.Println("State: Running -> Stopped")
            }
        case StateStopped:
            switch event {
            case EventReset:
                sm.state = StateIdle
                fmt.Println("State: Stopped -> Idle")
            case EventStart:
                sm.state = StateRunning
                fmt.Println("State: Stopped -> Running")
            }
        }
    }
}

func (sm *StateMachine) Stop() {
    close(sm.events)
    <-sm.done
}
```

### 20. Goroutine Recycling Pattern
**Purpose**: Reuse goroutines instead of creating new ones per task
**Use Case**: High-throughput systems where goroutine creation overhead matters
**LLM Benefit**: Encourages sustainable resource usage patterns

```go
type RecyclingPool struct {
    workers   chan func()
    workerWG  sync.WaitGroup
    closeOnce sync.Once
    closed    chan struct{}
}

func NewRecyclingPool(size int) *RecyclingPool {
    pool := &RecyclingPool{
        workers: make(chan func(), size),
        closed:  make(chan struct{}),
    }
    
    // Start persistent workers
    for i := 0; i < size; i++ {
        pool.workerWG.Add(1)
        go pool.worker()
    }
    
    return pool
}

func (p *RecyclingPool) worker() {
    defer p.workerWG.Done()
    
    for {
        select {
        case task := <-p.workers:
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        log.Printf("Worker recovered from panic: %v", r)
                    }
                }()
                task()
            }()
        case <-p.closed:
            return
        }
    }
}

func (p *RecyclingPool) Submit(task func()) bool {
    select {
    case p.workers <- task:
        return true
    case <-p.closed:
        return false
    default:
        return false // Pool full
    }
}

func (p *RecyclingPool) Close() {
    p.closeOnce.Do(func() {
        close(p.closed)
        p.workerWG.Wait()
    })
}
```

### 21. Rate Limiting Pattern (Ticker-Based)
**Purpose**: Strict interval-based rate limiting
**Use Case**: When precise timing intervals are required

```go
func rateLimiter(requests <-chan Request, rate time.Duration) <-chan Request {
    output := make(chan Request)
    ticker := time.NewTicker(rate)
    
    go func() {
        defer close(output)
        defer ticker.Stop()
        
        for req := range requests {
            <-ticker.C // Wait for rate limit
            output <- req
        }
    }()
    
    return output
}
```

### 22. Circuit Breaker Pattern
```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    failures    int
    lastFailure time.Time
    state       string // "closed", "open", "half-open"
    mu          sync.Mutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if cb.state == "open" {
        if time.Since(cb.lastFailure) < cb.timeout {
            return errors.New("circuit breaker open")
        }
        cb.state = "half-open"
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.maxFailures {
            cb.state = "open"
        }
        return err
    }
    
    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

### 23. Context-Aware Testing Pattern
**Purpose**: Prevent hanging tests due to goroutine deadlocks
**Use Case**: Testing concurrent code safely
**LLM Benefit**: Promotes robust test patterns for concurrent code

```go
func TestConcurrentFunction(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    done := make(chan struct{})
    var result string
    
    go func() {
        defer close(done)
        result = myConcurrentFunc()
    }()
    
    select {
    case <-done:
        if result != "expected" {
            t.Errorf("got %s, want expected", result)
        }
    case <-ctx.Done():
        t.Fatal("test timed out - possible deadlock")
    }
}

func TestConcurrentWithErrorHandling(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()
    
    errChan := make(chan error, 1)
    
    go func() {
        err := riskyOperation()
        errChan <- err
    }()
    
    select {
    case err := <-errChan:
        if err != nil {
            t.Errorf("operation failed: %v", err)
        }
    case <-ctx.Done():
        t.Fatal("operation timed out")
    }
}
```

### 24. Graceful Shutdown Pattern
```go
func gracefulShutdown() {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    server := &http.Server{Addr: ":8080"}
    
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    
    <-quit
    log.Println("Shutting down server...")
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
    
    log.Println("Server exited")
}
```

## Pattern Composition Examples

### 25. Combined Patterns for Real Systems

**Worker Pool + Context + Error Group**
```go
func robustProcessing(ctx context.Context, tasks []Task) error {
    g, ctx := errgroup.WithContext(ctx)
    
    // Create worker pool with context awareness
    jobs := make(chan Task, len(tasks))
    workerCount := 3
    
    // Start workers
    for i := 0; i < workerCount; i++ {
        g.Go(func() error {
            for {
                select {
                case task, ok := <-jobs:
                    if !ok {
                        return nil // Channel closed
                    }
                    if err := processTask(task); err != nil {
                        return err
                    }
                case <-ctx.Done():
                    return ctx.Err()
                }
            }
        })
    }
    
    // Send jobs
    go func() {
        defer close(jobs)
        for _, task := range tasks {
            select {
            case jobs <- task:
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return g.Wait()
}
```

**Fan-In + Circuit Breaker**
```go
func resilientFanIn(inputs ...<-chan string) <-chan string {
    output := make(chan string)
    cb := &CircuitBreaker{maxFailures: 3, timeout: 5 * time.Second}
    
    var wg sync.WaitGroup
    for i, input := range inputs {
        wg.Add(1)
        go func(id int, ch <-chan string) {
            defer wg.Done()
            for value := range ch {
                err := cb.Call(func() error {
                    select {
                    case output <- fmt.Sprintf("source-%d: %s", id, value):
                        return nil
                    default:
                        return errors.New("output channel blocked")
                    }
                })
                if err != nil {
                    log.Printf("Circuit breaker open for source %d: %v", id, err)
                }
            }
        }(i, input)
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

**Token Bucket + Bounded Parallelism**
```go
func rateLimitedParallelProcessing(tasks []Task, maxConcurrent int, bucket *TokenBucket) {
    sem := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    
    for _, task := range tasks {
        // Wait for token (rate limit)
        if !bucket.AcquireWithTimeout(5 * time.Second) {
            log.Printf("Rate limit timeout for task %d", task.ID)
            continue
        }
        
        // Acquire semaphore (concurrency limit)
        sem <- struct{}{}
        wg.Add(1)
        
        go func(t Task) {
            defer wg.Done()
            defer func() { <-sem }()
            
            processTask(t)
        }(task)
    }
    
    wg.Wait()
}
```

### Common Pitfalls to Avoid

1. **Goroutine Leaks**: Always ensure goroutines can exit
```go
// BAD: Goroutine may never exit
go func() {
    for {
        select {
        case data := <-ch:
            process(data)
        }
    }
}()

// GOOD: Provide exit condition
go func() {
    for {
        select {
        case data := <-ch:
            process(data)
        case <-done:
            return
        }
    }
}()
```

2. **Channel Deadlocks**: Ensure channels have readers/writers
```go
// BAD: Will deadlock
ch := make(chan int)
ch <- 1 // No reader available

// GOOD: Use buffered channel or separate goroutine
ch := make(chan int, 1)
ch <- 1
```

3. **Race Conditions**: Protect shared state
```go
// BAD: Race condition
var counter int
go func() { counter++ }()
go func() { counter++ }()

// GOOD: Use atomic operations or mutex
var counter int64
go func() { atomic.AddInt64(&counter, 1) }()
go func() { atomic.AddInt64(&counter, 1) }()
```

### Debugging Tools
- `go run -race`: Detect race conditions
- `go tool pprof`: Profile memory and CPU usage
- `go tool trace`: Analyze goroutine execution
- `GODEBUG=schedtrace=1000`: Monitor scheduler

## Best Practices for LLM Code Generation

1. **Always close channels** when done sending
2. **Use defer** for cleanup operations
3. **Handle context cancellation** in long-running operations
4. **Prefer channels over shared memory** for communication
5. **Use buffered channels** to prevent blocking when appropriate
6. **Implement timeouts** for external operations
7. **Check for channel closure** when receiving
8. **Use errgroup** for coordinated error handling
9. **Implement graceful shutdown** for services
10. **Profile and test** concurrent code thoroughly

## Pattern Selection Guide

| Use Case | Recommended Pattern | Alternative Patterns |
|----------|-------------------|---------------------|
| Data processing pipeline | Pipeline Pattern | Supervised Goroutines + Pipeline |
| Limited resource access | Worker Pool | Bounded Parallelism (Semaphore) |
| Simple load balancing | Shared Channel Fan-Out | Traditional Worker Pool |
| Merging multiple sources | Fan-In | Channel of Channels |
| Parallel processing | Fan-Out | Bounded Parallelism |
| Timed operations | Select with Timeout | Context with Timeout |
| Burst-friendly rate limiting | Token Bucket | Ticker-based Rate Limiting |
| Strict interval rate limiting | Ticker-based Rate Limiting | Token Bucket with small capacity |
| Shared state protection | Mutex/RWMutex | Channel-based State Machine |
| One-time initialization | sync.Once | Supervised initialization |
| Coordinated shutdown | Context + Graceful Shutdown | Channel-based shutdown signaling |
| External API calls | Circuit Breaker + Rate Limiting | Retry + Timeout patterns |
| Multiple goroutine coordination | WaitGroup + Error Group | Supervised Goroutines |
| Dynamic task routing | Channel of Channels | Fan-Out with routing logic |
| High-throughput processing | Goroutine Recycling | Worker Pool with persistent workers |
| Fault-tolerant goroutines | Recovery Wrapper | Supervised Goroutines |
| Complex state management | Channel-based State Machine | Mutex-based FSM |
| Test safety | Context-aware Testing | Timeout-based test helpers |
| Memory pressure handling | Backpressure patterns | Bounded channels with dropping |
| Priority-based processing | Priority Queue pattern | Multiple worker pools |

## LLM-Specific Guidelines for Pattern Selection

### When LLMs Should Prefer Simple Patterns:
- **Shared Channel Fan-Out** over dedicated channels when workers are identical
- **Bounded Parallelism** over full worker pools for simple concurrency limiting
- **Token Bucket** over complex rate limiting when burst handling is needed
- **Context cancellation** over manual shutdown signaling

### When LLMs Should Choose Advanced Patterns:
- **Channel of Channels** for dynamic routing requirements
- **Supervised Goroutines** when building hierarchical systems
- **State Machines** when logic has clear states and transitions
- **Pattern Composition** for production-grade systems

### Common LLM Antipatterns to Avoid:
1. Creating dedicated channels per worker when shared channels work better
2. Spawning unlimited goroutines without semaphore or pool limits
3. Using mutex-heavy solutions when channel-based patterns are cleaner
4. Ignoring context cancellation in long-running operations
5. Not implementing recovery wrappers for potentially panicking code
6. Creating worker pools for simple parallel tasks (use bounded parallelism instead)

This document serves as a comprehensive reference for implementing robust concurrent Go applications with special attention to patterns that help LLMs generate better, more production-ready concurrent code.