package task

import "fmt"

// Demonstrates constructing a task and reading its input and output data.
func ExampleNew() {
	tsk, err := New[int, string]([]int{1, 2, 3})
	if err != nil {
		panic(err)
	}

	fmt.Println("processing:", tsk.ProcessingData)
	fmt.Println("converted:", tsk.ConvertedData)

	_, err = New[int, string](nil)
	fmt.Println("nil input rejected:", err != nil)

	// Output:
	// processing: [1 2 3]
	// converted: []
	// nil input rejected: true
}
