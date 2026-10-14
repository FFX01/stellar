// Literals take their type from context (design §4).
fn literals() {
    let a: int32 = 5
    let b: int64 = 0xFF
    let c: int32 = a + 1
    let d = 5 // ERROR "cannot infer the type of integer literal 5"
    let e: int8 = 300 // ERROR "300 overflows int8"
    let f: int8 = -128
    let g: int8 = -129 // ERROR "-129 overflows int8"
    let h: uint8 = -1 // ERROR "-1 overflows uint8"
    let i: int32 = 1.5 // ERROR "cannot use float literal 1.5 as int32"
    let j: float64 = 5 // ERROR "integer literal 5 cannot be float64; write 5.0"
    let k: float64 = 5.0
    let l: float64 = k * 2.0
    let m = "hi" // ERROR "cannot infer the type of string literal"
    let n: str = "hi"
    let o = 1 + 2 // ERROR "cannot infer the type of integer literal 1"
    let p: int32 = 1 + 2
    let q: bool = true
    let r = q
    let s: ?int32 = 5 // ERROR "wrap it with Some(...)"
    let t: str = 5 // ERROR "cannot use integer literal 5 as str"
    let u: int64 = 1_000_000
    let v = a + b // ERROR "mismatched types int32 and int64"
    let w: bool = 1 < a
    let x = 1 < 2 // ERROR "cannot infer the type of integer literal 1"
    let y: int32 = -a
    let z: uint32 = 7
    let zz: uint32 = -z // ERROR "operator - is not defined for uint32"
}
