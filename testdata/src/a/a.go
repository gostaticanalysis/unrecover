package a

func f() {
	go func() { // want "this goroutine does not recover a panic"
		print("hello")
	}()

	//go func() { // -want "this goroutine does not recover a panic"
	//	defer recover() // NG
	//	print("hello")
	//}()

	go func() { // OK
		defer func() { recover() }()
		print("hello")
	}()

	go func() { // want "this goroutine does not recover a panic"
		recover()
		print("hello")
	}()

	go func() { // want "this goroutine does not recover a panic"
		defer func() { print("hello") }()
	}()
}
