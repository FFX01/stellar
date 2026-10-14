// Types, impls and interface conformance.
type Author object {
    firstName: str
    lastName: str
}

type Book object {
    title: str
    author: Author
    isbn: str
    tags: []str
    rating: int32
}

type Bad object {
    c: fn(mut Author)
    d: Map[Author, int32] // ERROR "map keys must be primitive types, not Author"
    e: Missing // ERROR "undefined type Missing"
    f: Result[int32, str] // ERROR "wrong number of type arguments for Result"
    h: Self // ERROR "Self is only allowed inside impl and interface declarations"
}

type Book object {} // ERROR "Book is already declared"
type Result object {} // ERROR "Result is a built-in name and cannot be redeclared"

type Sortable interface {
    fn sortKey(self) -> str
}

type Renamer interface {
    fn rename(mut self, name: str)
    fn clone(self) -> Self
}

type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
    Point() // ERROR "variant Point has no fields; remove the ()"
}

impl Sortable for Book {
    fn sortKey(self) -> str {
        return self.isbn
    }
}

impl Sortable for Book { // ERROR "Book already implements Sortable"
}

impl Renamer for Author { // ERROR "Author does not implement Renamer: missing method clone"
    fn rename(self, name: str) { // ERROR "want fn(mut self, str), have fn(self, str)"
    }
    pub fn extra(self) { // ERROR "remove pub" ERROR "extra is not a method of Renamer"
    }
}

impl Renamer for Book {
    fn rename(mut self, name: str) {
        self.title = name
    }
    fn clone(self) -> Book {
        return self
    }
}

impl Book {
    pub fn label(self) -> str {
        return self.title + " by " + self.author.lastName
    }
    fn sortKey(self) -> str { // ERROR "Book.sortKey conflicts with Sortable.sortKey"
        return ""
    }
    fn title(self) -> str { // ERROR "Book has both a field and a method named title"
        return ""
    }
    fn label(self) -> str { // ERROR "Book.label is already declared"
        return ""
    }
}

impl int32 { // ERROR "cannot define methods on built-in type int32"
}

impl Error for int32 { // ERROR "cannot implement built-in interface Error for built-in type int32"
    fn message(self) -> str {
        return "x"
    }
}

type NotFound object {
    key: str
}

impl Error for NotFound {
    fn message(self) -> str {
        return "not found: " + self.key
    }
}

impl Sortable for Sortable { // ERROR "cannot implement methods for Sortable"
}

fn takesSelf(self) { // ERROR "self is only allowed as the first parameter of a method"
}

fn main(a: int32) { // ERROR "main must be declared as fn main() or fn main() -> Result"
}
