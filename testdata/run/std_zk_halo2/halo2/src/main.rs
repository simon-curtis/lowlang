mod circuit;
mod records;
mod util;

use std::path::PathBuf;

fn verify_mode(path: &str) -> ! {
    let text = match std::fs::read_to_string(path) {
        Ok(t) => t,
        Err(e) => {
            println!("rejected: cannot read {path}: {e}");
            std::process::exit(1);
        }
    };
    let proof = match util::unhex(&text) {
        Ok(p) => p,
        Err(e) => {
            println!("rejected: bad hex: {e}");
            std::process::exit(1);
        }
    };
    let (params, vk) = circuit::setup();
    match circuit::verify_bytes(&params, &vk, &proof) {
        Ok(_) => {
            println!("accepted");
            std::process::exit(0);
        }
        Err(e) => {
            println!("rejected: {e}");
            std::process::exit(1);
        }
    }
}

fn low_proof_path() -> Option<PathBuf> {
    [
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("low_proof.hex"),
        PathBuf::from("low_proof.hex"),
    ]
    .into_iter()
    .find(|p| p.exists())
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args.len() == 3 && args[1] == "verify" {
        verify_mode(&args[2]);
    }
    if args.len() != 1 {
        eprintln!("usage: halo2-ref [verify <proof hex file>]");
        std::process::exit(2);
    }
    let mut out = util::Out::default();
    records::all(&mut out);
    circuit::records(&mut out);
    for l in &out.lines {
        println!("{l}");
    }
    if let Some(path) = low_proof_path() {
        let verdict = std::fs::read_to_string(&path)
            .map_err(|e| e.to_string())
            .and_then(|t| util::unhex(&t))
            .and_then(|proof| {
                let (params, vk) = circuit::setup();
                circuit::verify_bytes(&params, &vk, &proof).map(|_| ())
            });
        match verdict {
            Ok(()) => println!("low_proof_verdict: accepted"),
            Err(e) => {
                eprintln!("low proof rejected: {e}");
                println!("low_proof_verdict: rejected");
            }
        }
    }
}
