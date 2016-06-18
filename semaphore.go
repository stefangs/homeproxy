package main

type empty struct {}
type Semaphore chan empty
func (s Semaphore) P(n int) {
	e := empty{}
	for i := 0; i < n; i++ {
		s <- e
	}
}

func (s Semaphore) V(n int) {
	for i := 0; i < n; i++ {
		<-s
	}
}


