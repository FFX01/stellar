// Value semantics: let/var, mut parameters, call-site markers (design §5).
type Author object {
    firstName: str
}

type Book object {
    title: str
    author: Author
    tags: []str
}

impl Book {
    pub fn label(self) -> str {
        return self.title
    }
    pub fn retitle(mut self, t: str) {
        self.title = t
    }
    pub fn rename(self, t: str) {
        self.title = t // ERROR "the method takes self; declare it with mut self"
    }
}

type Shelf object {
    books: []Book
}

fn rename(b: mut Book, t: str) {
    b.title = t
    b.author.firstName = "x"
    b = Book{ title: t, author: b.author, tags: [] }
}

fn swap(a: mut Book, b: mut Book) {
}

fn getBook() -> Book {
    return Book{ title: "t", author: Author{ firstName: "a" }, tags: [] }
}

fn describe(b: Book) -> str {
    b.title = "x" // ERROR "parameter b is not mut; declare it as b: mut Book"
    return b.title
}

fn bump(n: mut int32) {
    n = n + 1
}

fn values(a: Author, books: []Book) {
    var b = Book{ title: "Dune", author: a, tags: [] }
    let fixed = Book{ title: "Fixed", author: a, tags: [] }
    var shelf = Shelf{ books: [b] }

    b.title = "x"
    b.author.firstName = "y"
    b.tags[0] = "z"
    fixed.title = "x" // ERROR "cannot assign to fixed.title: fixed is declared with let; use var"

    rename(mut b, "x")
    rename(b, "x") // ERROR "rename may change argument 1; mark it: mut b"
    rename(mut fixed, "x") // ERROR "cannot pass fixed as mut: fixed is declared with let"
    rename(mut getBook(), "x") // ERROR "getBook() is not a variable, field or element"
    rename(mut shelf.books[0], "x")
    let _ = describe(mut b) // ERROR "mut can only mark an argument to a mut parameter"
    let d: str = describe(b)

    mut b.retitle("x")
    b.retitle("x") // ERROR "retitle changes b; mark the call: mut b.retitle(...)"
    mut fixed.retitle("x") // ERROR "cannot call retitle on fixed: fixed is declared with let"
    let _ = mut b.label() // ERROR "label does not change b; remove mut"
    let l: str = b.label()
    mut shelf.books[0].retitle("y")
    mut getBook().retitle("x") // ERROR "getBook() is not a variable, field or element"
    let f = b.retitle // ERROR "retitle changes its receiver, so it cannot be used as a value"
    let g: fn(mut Book, str) = Book.retitle
    Book.retitle(mut b, "x")

    swap(mut b, mut b) // ERROR "b overlaps another mut argument of this call"
    swap(mut shelf.books[0], mut shelf.books[1]) // ERROR "overlaps another mut argument"
    swap(mut b, mut shelf.books[0])
    rename(mut shelf.books[0], b.title)

    var n: int32 = 1
    bump(mut n)
    var wide: int64 = 1
    bump(mut wide) // ERROR "cannot pass wide (type int64) as mut int32"

    for book in books {
        book.title = "x" // ERROR "loop variable book is a copy; index the collection instead"
    }
    var xs: []int32 = [1]
    push(mut xs, 2)
    push(xs, 2) // ERROR "push changes its first argument; mark it: mut xs"
    let ys: []int32 = [1]
    push(mut ys, 2) // ERROR "ys is declared with let"
    xs[0] = 5
    ys[0] = 5 // ERROR "cannot assign to ys[0]: ys is declared with let"

    var ages: Map[str, int32] = [:]
    ages["a"] = 1
    getBook().title = "x" // ERROR "cannot assign to getBook().title"
}
