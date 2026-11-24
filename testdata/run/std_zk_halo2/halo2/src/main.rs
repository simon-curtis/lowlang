mod circuit;
mod records;
mod regions;
mod util;

use std::path::PathBuf;

fn read_proof(path: &PathBuf) -> Result<Vec<u8>, String> {
    std::fs::read_to_string(path)
        .map_err(|e| format!("cannot read {}: {e}", path.display()))
        .and_then(|t| util::unhex(&t))
}

fn verify_mode(path: &str) -> ! {
    let verdict = read_proof(&PathBuf::from(path)).and_then(|proof| {
        let (params, vk) = circuit::setup();
        circuit::verify_bytes(&params, &vk, &proof).map(|_| ())
    });
    match verdict {
        Ok(()) => {
            println!("accepted");
            std::process::exit(0);
        }
        Err(e) => {
            println!("rejected: {e}");
            std::process::exit(1);
        }
    }
}

fn checked_in(name: &str) -> PathBuf {
    [PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(name), PathBuf::from(name)]
        .into_iter()
        .find(|p| p.exists())
        .unwrap_or_else(|| {
            eprintln!("{name} is missing");
            std::process::exit(1);
        })
}

/// Records that the reference verifier accepts a proof Low made, with the
/// Blake2b-256 digest of the proof it judged, and fails when it does not.
fn verdict(out: &mut util::Out, name: &str, file: &str, check: impl Fn(&[u8]) -> Result<(), String>) {
    let proof = read_proof(&checked_in(file)).unwrap_or_else(|e| {
        eprintln!("{e}");
        std::process::exit(1);
    });
    if let Err(e) = check(&proof) {
        eprintln!("{file} rejected: {e}");
        std::process::exit(1);
    }
    let digest = blake2b_simd::Params::new().hash_length(32).hash(&proof);
    out.rec(name, vec!["accepted".to_string(), util::hex(digest.as_bytes())]);
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
    regions::records(&mut out);
    verdict(&mut out, "low_proof_verdict", "low_proof.hex", |proof| {
        let (params, vk) = circuit::setup();
        circuit::verify_bytes(&params, &vk, proof).map(|_| ())
    });
    verdict(&mut out, "regions_low_proof_verdict", "low_regions_proof.hex", |proof| {
        let (params, vk) = regions::setup();
        regions::verify_bytes(&params, &vk, proof)
    });
    for l in &out.lines {
        println!("{l}");
    }
}
