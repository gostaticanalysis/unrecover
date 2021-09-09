package a

func f() { // want f:"panicable"
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

	go p1() // OK
	go p2() // want "this goroutine does not recover a panic"
}

func p1() { // want p1:"panicable"
	defer func() { recover() }()
	panic("hello")
}

func p2() { // want p2:"panicable"
	panic("hello")
}
