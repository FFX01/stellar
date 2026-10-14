// Top-level declarations: imports, objects, interfaces, enums, impls.
import "lib/people"
import "lib/sorting"

pub type Book object {
    pub title: str
    pub author: people.Author
    isbn: str
}

type Empty object {}

pub type Sortable interface {
    fn sortKey(self) -> str
}

pub type Speaker interface {
    fn hello(self) -> str
    fn rename(mut self, name: str)
}

pub type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
}

impl Sortable for Book {
    fn sortKey(self) -> str {
        return self.isbn
    }
}

impl sorting.Ranked for Book {
    fn rank(self) -> int32 {
        return 1
    }
}

impl Book {
    pub fn new(title: str, author: people.Author, isbn: str) -> Book {
        return Book{ title: title, author: author, isbn: isbn }
    }

    pub fn label(self) -> str {
        return self.title + " by " + self.author.lastName
    }

    pub fn retitle(mut self, t: str) {
        self.title = t
    }

    fn checksum(self) -> int32 {
        return 0
    }
}

pub fn add(a: int32, b: int32) -> Result[int32] {
    if a > 100 or b > 100 {
        return Err("Both values must be < 100")
    }
    return Ok(a + b)
}

fn main() {
    let b = Book.new("Dune", people.Author{ firstName: "Frank", lastName: "Herbert" }, "978-0441013593")
    print(b.label())
}
