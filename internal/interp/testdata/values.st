// Value semantics: copies on assignment, mut parameters, mut self methods.
type Author object {
    name: str
}

type Book object {
    title: str
    author: Author
    tags: []str
}

impl Book {
    pub fn retitle(mut self, t: str) {
        self.title = t
    }
    pub fn label(self) -> str {
        return self.title + " by " + self.author.name
    }
    pub fn tag(mut self, t: str) {
        push(mut self.tags, t)
    }
}

type Shelf object {
    books: []Book
}

fn rename(b: mut Book, t: str) {
    b.title = t
}

fn renameCopy(b: Book, t: str) -> Book {
    var c = b
    c.title = t
    return c
}

fn bump(n: mut int32) {
    n = n + 1
}

fn swap(a: mut int32, b: mut int32) {
    let t = a
    a = b
    b = t
}

fn main() {
    var b = Book{ title: "Dune", author: Author{ name: "Herbert" }, tags: [] }
    let snapshot = b
    let shelf: []Book = [b]
    b.title = "Dune Messiah"
    b.author.name = "F. Herbert"
    print("b: " + b.label())
    print("snapshot: " + snapshot.label())
    print("shelf[0]: " + (try shelf[0]).label())

    rename(mut b, "Children of Dune")
    print("after rename: " + b.title)
    let other = renameCopy(b, "God Emperor")
    print("renameCopy: " + other.title + ", original: " + b.title)

    mut b.retitle("Heretics")
    mut b.tag("sf")
    mut b.tag("classic")
    print("retitle: " + b.title + ", tags: " + str(len(b.tags)) + ", snapshot tags: " + str(len(snapshot.tags)))

    var s = Shelf{ books: [snapshot, b] }
    let before = s
    mut s.books[0].retitle("Dune (2nd ed.)")
    rename(mut s.books[1], "Chapterhouse")
    print("shelf: " + (try s.books[0]).title + " / " + (try s.books[1]).title)
    print("before: " + (try before.books[0]).title + " / " + (try before.books[1]).title)

    var n: int32 = 1
    bump(mut n)
    bump(mut n)
    print("n " + str(n))
    var x: int32 = 1
    var y: int32 = 2
    swap(mut x, mut y)
    print("swapped " + str(x) + " " + str(y))

    var xs: []int32 = [1, 2, 3]
    let ys = xs
    xs[0] = 100
    push(mut xs, 4)
    print("xs " + str(len(xs)) + " first " + str(try xs[0]) + "; ys " + str(len(ys)) + " first " + str(try ys[0]))

    var counts: Map[str, int32] = [:]
    let words: []str = ["a", "b", "a"]
    for word in words {
        let current: int32 = try counts[word] else 0
        counts[word] = current + 1
    }
    print("a=" + str(try counts["a"]) + " b=" + str(try counts["b"]))

    var grid: []Book = [snapshot, snapshot]
    for i in 0..len(grid) {
        grid[i].title = "copy " + str(i)
    }
    print("grid: " + (try grid[0]).title + ", " + (try grid[1]).title + "; snapshot: " + snapshot.title)

    var loop: []Book = [snapshot]
    for book in loop {
        let t = book.title
        loop[0].title = "changed during loop"
        print("loop var still: " + book.title + " (" + t + ")")
    }
    print("loop[0]: " + (try loop[0]).title)
}
