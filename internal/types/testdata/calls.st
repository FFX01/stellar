// Calls, methods, enum variants, conversions and built-ins.
type Author object {
    firstName: str
    lastName: str
}

type Book object {
    title: str
    author: Author
    isbn: str
    onSave: fn()
}

type Sortable interface {
    fn sortKey(self) -> str
}

type Ranked interface {
    fn sortKey(self) -> str
}

impl Sortable for Book {
    fn sortKey(self) -> str {
        return self.isbn
    }
}

impl Ranked for Book {
    fn sortKey(self) -> str {
        return self.title
    }
}

impl Book {
    pub fn new(title: str, author: Author, isbn: str, onSave: fn()) -> Book {
        return Book{ title: title, author: author, isbn: isbn, onSave: onSave }
    }
    pub fn label(self) -> str {
        return self.title + " by " + self.author.lastName
    }
    pub fn retitle(mut self, t: str) {
        self.title = t
    }
}

type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
}

fn add(a: int32, b: int32) -> int32 {
    return a + b
}

fn calls(b: Book, m: mut Book, a: Author) {
    var v2 = b
    let s: int32 = add(1, 2)
    let t = add(1) // ERROR "wrong number of arguments in call to add: want 2, got 1"
    let u = add(a: 1, b: 2) // ERROR "named arguments are only allowed" ERROR "named arguments are only allowed"
    let v = add(1, "x") // ERROR "cannot use string literal \"x\" as int32"
    add(1, 2) // ERROR "unused value of type int32"
    let w = undefinedFn(1) // ERROR "undefined: undefinedFn"
    let x = s(1) // ERROR "cannot call s (type int32)"

    let label: str = b.label()
    mut m.retitle("New")
    mut b.retitle("New") // ERROR "parameter b is not mut"
    let l2 = m.label()
    let key = b.sortKey() // ERROR "sortKey is ambiguous: it is declared by Ranked and Sortable"
    let key2: str = Sortable.sortKey(b)
    let key3: str = Ranked.sortKey(b)
    let f: fn(Book) -> str = Book.label
    let g = b.label
    let h: str = g()
    let nb = Book.new("t", a, "i", fn() {})
    let bad = b.new("t", a, "i", fn() {}) // ERROR "new is a static function; call it as Book.new(...)"
    let missing = Book.nothing() // ERROR "Book has no function or method named nothing"
    b.onSave()
    mut b.onSave() // ERROR "mut can only mark a call to a mut self method"
    let objCall = Book("x") // ERROR "Book is a type; construct it with Book{...}"

    let c = Shape.Circle(r: 1.5)
    let r = Shape.Rect(w: 1.0, h: 2.0)
    let e = Shape.Empty
    let e2 = Shape.Empty() // ERROR "Shape.Empty has no fields; remove the ()"
    let c2 = Shape.Circle // ERROR "Shape.Circle has fields; construct it with Shape.Circle(r: ...)"
    let c3 = Shape.Circle(1.5) // ERROR "variant fields must be named: Shape.Circle(r: ...)" ERROR "missing field r in Shape.Circle"
    let c4 = Shape.Rect(w: 1.0) // ERROR "missing field h in Shape.Rect"
    let c5 = Shape.Circle(r: 1.0, d: 2.0) // ERROR "Shape.Circle has no field d"
    let c6 = Shape.Square // ERROR "Shape has no variant, function or method named Square"
}

fn conversions(n: int32, f: float64, s: str, ok: bool) {
    let a: int64 = int64(n)
    let b = int64(5)
    let c: float64 = float64(n)
    let d: int32 = int32(f)
    let e: str = str(n)
    let g: str = str(ok)
    let h = int32(s) // ERROR "cannot convert str to int32; parsing strings is not supported yet"
    let i = bool(n) // ERROR "cannot convert n (type int32) to bool"
    let j = float64(5) // ERROR "integer literal 5 cannot be float64"
    let k = int32(1, 2) // ERROR "int32 takes 1 argument, got 2"
}

fn builtins(xs: []int32, ys: mut []int32, b: Book, a: Author) {
    print("hello")
    print(5) // ERROR "cannot use integer literal 5 as str"
    let n: int64 = len(xs)
    let n2 = len(5) // ERROR "cannot infer the type of integer literal"
    push(mut ys, 4)
    push(mut xs, 4) // ERROR "parameter xs is not mut"
    let gone = copy(b) // ERROR "undefined: copy"
    let p = panic("boom")
}

fn results(flag: bool) -> Result[int32] {
    if flag {
        return Err("bad flag")
    }
    let e: Error = wrap(Err("x"), "ctx") // ERROR "cannot use Err(...) as Error"
    return Ok(1)
}

fn bare() -> Result {
    return Ok()
}

fn bareWrong() -> Result {
    return Ok(1) // ERROR "Ok takes no value here"
}

fn maybes(books: []Book, isbn: str) -> ?Book {
    for b in books {
        if b.isbn == isbn {
            return Some(b)
        }
    }
    let n: ?int32 = None
    let m = None // ERROR "cannot infer the type of None"
    let o = Some(5) // ERROR "cannot infer the type of integer literal 5"
    let p: ?int32 = Some(5)
    return None
}

fn handling(books: []Book) -> Result[int32] {
    let sum = try results(true)
    let sum2: int32 = try results(true) else 0
    let sum3: int32 = try results(true) else |e| {
        return Err(wrap(e, "adding"))
    }
    let c: ?Error = cause(wrap(sum2, "x")) // ERROR "cannot use sum2 (type int32) as Error"
    let first = try books[0] else {
        return Err("no books")
    }
    let x = try maybes(books, "x") else |e| { // ERROR "None carries no error; remove |e|"
        return Err("none")
    }
    try bare()
    try bare() else |e| {
        print(e.message())
    }
    results(true) // ERROR "unhandled Result from results(true): handle it with try or match"
    let _ = results(true) // ERROR "a Result cannot be discarded"
    let _ = try results(true)
    let _ = maybes(books, "y")
    let notTry = try sum // ERROR "try needs a Result or Maybe, but sum has type int32"
    let v: int32 = results(false) // ERROR "use try to unwrap it"
    return Err(NotFound{ key: "x" })
}

type NotFound object {
    key: str
}

impl Error for NotFound {
    fn message(self) -> str {
        return "not found: " + self.key
    }
}

fn errs() -> Result[int32] {
    return Err(5) // ERROR "cannot use integer literal 5 as str"
}

fn errs2(b: Book) -> Result[int32] {
    return Err(b) // ERROR "Err needs a str message or an Error, not Book"
}
