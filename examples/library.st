// A small library catalogue exercising most of the language.

pub type Author object {
    pub firstName: str
    pub lastName: str
}

pub type Book object {
    pub title: str
    pub author: Author
    pub isbn: str
    pub rating: int32
}

pub type Sortable interface {
    fn sortKey(self) -> str
}

impl Sortable for Book {
    fn sortKey(self) -> str {
        return self.isbn
    }
}

impl Book {
    pub fn new(title: str, author: Author, isbn: str) -> Book {
        return Book{ title: title, author: author, isbn: isbn, rating: 0 }
    }

    pub fn label(self) -> str {
        return self.title + " by " + self.author.firstName + " " + self.author.lastName
    }

    pub fn rate(mut self, stars: int32) -> Result {
        if stars < 0 or stars > 5 {
            return Err(InvalidRating{ stars: stars })
        }
        self.rating = stars
        return Ok()
    }
}

pub type InvalidRating object {
    pub stars: int32
}

impl Error for InvalidRating {
    fn message(self) -> str {
        return "rating must be between 0 and 5, got " + str(self.stars)
    }
}

pub type Shelf enum {
    Fiction(genre: str)
    Reference
    Archive(year: int32)
}

fn shelfName(s: Shelf) -> str {
    return match s {
        Shelf.Fiction(genre) => "fiction/" + genre
        Shelf.Reference => "reference"
        Shelf.Archive(year) => "archive/" + str(year)
    }
}

fn findByIsbn(books: []Book, isbn: str) -> ?Book {
    for b in books {
        if b.isbn == isbn {
            return Some(b)
        }
    }
    return None
}

fn keys(items: []Sortable) -> str {
    var out: str = ""
    for item in items {
        out = out + item.sortKey() + " "
    }
    return out
}

fn best(books: []Book) -> ?Book {
    var top = try books[0] else {
        return None
    }
    for b in books {
        if b.rating > top.rating {
            top = b
        }
    }
    return Some(top)
}

fn main() -> Result {
    let herbert = Author{ firstName: "Frank", lastName: "Herbert" }
    let leGuin = Author{ firstName: "Ursula", lastName: "Le Guin" }

    var dune = Book.new("Dune", herbert, "978-0441013593")
    try mut dune.rate(5) else |e| {
        return Err(wrap(e, "rating Dune"))
    }

    let books: []Book = [dune, Book.new("The Dispossessed", leGuin, "978-0061054884")]
    print(keys(books))

    match findByIsbn(books, "978-0061054884") {
        Some(b) => print("found " + b.label())
        None => print("not found")
    }

    let favourite = try best(books) else {
        return Err("no books")
    }
    print("best: " + favourite.title + " (" + str(favourite.rating) + " stars)")

    let shelf = Shelf.Fiction(genre: "sf")
    let describeShelf = fn(n: int64) -> str {
        return str(n) + " books on " + shelfName(shelf)
    }
    print(describeShelf(len(books)))

    var ratings: Map[str, int32] = [:]
    for b in books {
        ratings[b.isbn] = b.rating
    }
    let duneRating: int32 = try ratings["978-0441013593"] else 0
    print("dune rating: " + str(duneRating))
    return Ok()
}
