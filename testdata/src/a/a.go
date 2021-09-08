package a

func f() {
	go func() { // want "this goroutine does not recover a panic"
	}()

	//go func() { // -want "this goroutine does not recover a panic"
	//	defer recover() // NG
	//}()

	go func() { // OK
		defer func() { recover() }()
	}()

	go func() { // want "this goroutine does not recover a panic"
		recover()
	}()
}
