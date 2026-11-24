use ff::PrimeField;
use halo2_proofs::arithmetic::CurveAffine;

pub fn hex(bytes: &[u8]) -> String {
    let mut s = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        s.push_str(&format!("{:02x}", b));
    }
    s
}

pub fn unhex(text: &str) -> Result<Vec<u8>, String> {
    let text = text.trim();
    if text.len() % 2 != 0 {
        return Err("odd hex length".to_string());
    }
    (0..text.len() / 2)
        .map(|i| u8::from_str_radix(&text[2 * i..2 * i + 2], 16).map_err(|e| e.to_string()))
        .collect()
}

pub fn fe<F: PrimeField>(f: &F) -> String {
    hex(f.to_repr().as_ref())
}

pub fn pt<C: CurveAffine>(p: &C) -> String {
    hex(p.to_bytes().as_ref())
}

pub fn modulus_le<F: PrimeField>() -> [u8; 32] {
    let digits = F::MODULUS.trim_start_matches("0x");
    let padded = format!("{:0>64}", digits);
    let mut bytes = unhex(&padded).unwrap();
    bytes.reverse();
    bytes.try_into().unwrap()
}

pub fn add_one_le(mut v: [u8; 32]) -> [u8; 32] {
    for b in v.iter_mut() {
        let (n, carry) = b.overflowing_add(1);
        *b = n;
        if !carry {
            break;
        }
    }
    v
}

pub fn sub_one_le(mut v: [u8; 32]) -> [u8; 32] {
    for b in v.iter_mut() {
        let (n, borrow) = b.overflowing_sub(1);
        *b = n;
        if !borrow {
            break;
        }
    }
    v
}

pub fn repr_from_bytes<F: PrimeField>(bytes: &[u8; 32]) -> F::Repr {
    let mut r = F::Repr::default();
    r.as_mut().copy_from_slice(bytes);
    r
}

#[derive(Default)]
pub struct Out {
    pub lines: Vec<String>,
}

impl Out {
    pub fn rec(&mut self, name: &str, items: Vec<String>) {
        self.lines.push(format!("{}: {}", name, items.join(" ")));
    }
}
