use ff::{Field, FromUniformBytes, PrimeField};
use group::Curve;
use halo2_proofs::arithmetic::CurveAffine;
use halo2_proofs::circuit::{Layouter, SimpleFloorPlanner, Value};
use halo2_proofs::pasta::{EqAffine, Fp, Fq};
use halo2_proofs::plonk::{
    create_proof, keygen_pk, keygen_vk, verify_proof, Advice, Circuit, Column, ConstraintSystem,
    Error, Fixed, Instance, SingleVerifier, TableColumn, VerifyingKey,
};
use halo2_proofs::poly::commitment::{Blind, Params};
use halo2_proofs::poly::Rotation;
use halo2_proofs::transcript::{
    Blake2bRead, Blake2bWrite, Challenge255, Transcript, TranscriptRead,
};
use rand_core::OsRng;
use std::io;

use crate::util::*;

pub const K: u32 = 4;
pub const INSTANCE: u64 = 56;

#[derive(Clone)]
pub struct Config {
    a: Column<Advice>,
    b: Column<Advice>,
    c: Column<Advice>,
    q_mul: Column<Fixed>,
    q_rot: Column<Fixed>,
    q_look: Column<Fixed>,
    t: TableColumn,
    inst: Column<Instance>,
}

#[derive(Clone, Default)]
pub struct RefCircuit;

impl Circuit<Fp> for RefCircuit {
    type Config = Config;
    type FloorPlanner = SimpleFloorPlanner;

    fn without_witnesses(&self) -> Self {
        self.clone()
    }

    fn configure(meta: &mut ConstraintSystem<Fp>) -> Config {
        let a = meta.advice_column();
        let b = meta.advice_column();
        let c = meta.advice_column();
        let q_mul = meta.fixed_column();
        let q_rot = meta.fixed_column();
        let q_look = meta.fixed_column();
        let t = meta.lookup_table_column();
        let inst = meta.instance_column();
        meta.enable_equality(a);
        meta.enable_equality(b);
        meta.enable_equality(c);
        meta.enable_equality(inst);

        meta.create_gate("mul", |m| {
            let q = m.query_fixed(q_mul);
            let a = m.query_advice(a, Rotation::cur());
            let b = m.query_advice(b, Rotation::cur());
            let c = m.query_advice(c, Rotation::cur());
            vec![q * (a * b - c)]
        });
        meta.create_gate("rot", |m| {
            let q = m.query_fixed(q_rot);
            let a_next = m.query_advice(a, Rotation::next());
            let b = m.query_advice(b, Rotation::cur());
            let c = m.query_advice(c, Rotation::cur());
            vec![q * (a_next - b - c * Fp::from(5))]
        });
        meta.lookup(|m| {
            let q = m.query_fixed(q_look);
            let a = m.query_advice(a, Rotation::cur());
            vec![(q * a, t)]
        });

        Config { a, b, c, q_mul, q_rot, q_look, t, inst }
    }

    fn synthesize(&self, config: Config, mut layouter: impl Layouter<Fp>) -> Result<(), Error> {
        let one = Fp::ONE;
        let v = |x: u64| Value::known(Fp::from(x));
        let c_row2 = layouter.assign_region(
            || "main",
            |mut region| {
                let a = config.a;
                let b = config.b;
                let c = config.c;
                // (a, b, c) per row, selectors below
                region.assign_advice(|| "", a, 0, || v(3))?;
                region.assign_advice(|| "", b, 0, || v(4))?;
                let c0 = region.assign_advice(|| "", c, 0, || v(12))?;
                region.assign_fixed(|| "", config.q_mul, 0, || Value::known(one))?;

                region.assign_advice(|| "", a, 1, || v(5))?;
                region.assign_advice(|| "", b, 1, || v(6))?;
                region.assign_advice(|| "", c, 1, || v(30))?;
                region.assign_fixed(|| "", config.q_mul, 1, || Value::known(one))?;

                region.assign_advice(|| "", a, 2, || v(7))?;
                region.assign_advice(|| "", b, 2, || v(8))?;
                let c2 = region.assign_advice(|| "", c, 2, || v(56))?;
                region.assign_fixed(|| "", config.q_mul, 2, || Value::known(one))?;

                region.assign_advice(|| "", a, 3, || v(0))?;
                region.assign_advice(|| "", b, 3, || v(2))?;
                region.assign_advice(|| "", c, 3, || v(3))?;
                region.assign_fixed(|| "", config.q_rot, 3, || Value::known(one))?;

                let a4 = region.assign_advice(|| "", a, 4, || v(17))?;
                region.assign_advice(|| "", b, 4, || v(0))?;
                region.assign_advice(|| "", c, 4, || v(0))?;
                region.assign_fixed(|| "", config.q_look, 4, || Value::known(one))?;

                region.assign_advice(|| "", a, 5, || v(5))?;
                region.assign_advice(|| "", b, 5, || v(0))?;
                region.assign_advice(|| "", c, 5, || v(0))?;
                region.assign_fixed(|| "", config.q_look, 5, || Value::known(one))?;

                let a6 = region.assign_advice(|| "", a, 6, || v(12))?;
                let b7 = region.assign_advice(|| "", b, 7, || v(12))?;
                let c8 = region.assign_advice(|| "", c, 8, || v(17))?;

                region.constrain_equal(c0.cell(), a6.cell())?;
                region.constrain_equal(a6.cell(), b7.cell())?;
                region.constrain_equal(a4.cell(), c8.cell())?;
                Ok(c2)
            },
        )?;
        layouter.assign_table(
            || "t",
            |mut table| {
                for (i, x) in [0u64, 3, 5, 17, 30].iter().enumerate() {
                    table.assign_cell(|| "", config.t, i, || Value::known(Fp::from(*x)))?;
                }
                Ok(())
            },
        )?;
        layouter.constrain_instance(c_row2.cell(), config.inst, 0)
    }
}

/// Wraps a transcript and logs the bytes of every challenge it squeezes.
pub struct Logged<T> {
    pub inner: T,
    pub log: Vec<[u8; 32]>,
}

impl<T: Transcript<EqAffine, Challenge255<EqAffine>>> Transcript<EqAffine, Challenge255<EqAffine>>
    for Logged<T>
{
    fn squeeze_challenge(&mut self) -> Challenge255<EqAffine> {
        let c = self.inner.squeeze_challenge();
        self.log.push(*c);
        c
    }
    fn common_point(&mut self, point: EqAffine) -> io::Result<()> {
        self.inner.common_point(point)
    }
    fn common_scalar(&mut self, scalar: Fp) -> io::Result<()> {
        self.inner.common_scalar(scalar)
    }
}

impl<T: TranscriptRead<EqAffine, Challenge255<EqAffine>>>
    TranscriptRead<EqAffine, Challenge255<EqAffine>> for Logged<T>
{
    fn read_point(&mut self) -> io::Result<EqAffine> {
        self.inner.read_point()
    }
    fn read_scalar(&mut self) -> io::Result<Fp> {
        self.inner.read_scalar()
    }
}

pub fn setup() -> (Params<EqAffine>, VerifyingKey<EqAffine>) {
    let params = Params::<EqAffine>::new(K);
    let vk = keygen_vk(&params, &RefCircuit).expect("keygen_vk");
    (params, vk)
}

/// Verifies against the instance [56]; also rejects bytes left over after the
/// proof, which the crate's verifier ignores. Returns the verifier's challenges.
pub fn verify_bytes(
    params: &Params<EqAffine>,
    vk: &VerifyingKey<EqAffine>,
    proof: &[u8],
) -> Result<Vec<[u8; 32]>, String> {
    let mut rest = proof;
    let inst = [Fp::from(INSTANCE)];
    let cols: [&[Fp]; 1] = [&inst];
    let all: [&[&[Fp]]; 1] = [&cols];
    let log = {
        let inner = Blake2bRead::<&mut &[u8], EqAffine, Challenge255<EqAffine>>::init(&mut rest);
        let mut t = Logged { inner, log: vec![] };
        verify_proof(params, vk, SingleVerifier::new(params), &all, &mut t)
            .map_err(|e| format!("{e}").replace('\n', " "))?;
        t.log
    };
    if !rest.is_empty() {
        return Err(format!("{} trailing bytes after proof", rest.len()));
    }
    Ok(log)
}

fn parse_points(text: &str) -> Vec<EqAffine> {
    text.split('(')
        .skip(1)
        .map(|s| {
            let s = s.split(')').next().unwrap();
            let mut parts = s.split(", ");
            let coord = |p: &str| -> Fq {
                let mut bytes = unhex(p.trim_start_matches("0x")).unwrap();
                bytes.reverse();
                Option::from(Fq::from_repr(repr_from_bytes::<Fq>(&bytes.try_into().unwrap())))
                    .unwrap()
            };
            let x = coord(parts.next().unwrap());
            let y = coord(parts.next().unwrap());
            Option::from(EqAffine::from_xy(x, y)).expect("point on curve")
        })
        .collect()
}

fn bracket_after<'a>(text: &'a str, marker: &str) -> &'a str {
    let start = text.find(marker).expect("marker") + marker.len();
    let rest = &text[start..];
    &rest[..rest.find(']').unwrap()]
}

fn number_after(text: &str, marker: &str) -> usize {
    let start = text.find(marker).expect("marker") + marker.len();
    text[start..]
        .chars()
        .take_while(|c| c.is_ascii_digit())
        .collect::<String>()
        .parse()
        .unwrap()
}

fn fixed_columns(usable_rows: usize) -> Vec<Vec<Fp>> {
    let mut cols = vec![vec![Fp::ZERO; 1 << K]; 4];
    for r in [0usize, 1, 2] {
        cols[0][r] = Fp::ONE;
    }
    cols[1][3] = Fp::ONE;
    for r in [4usize, 5] {
        cols[2][r] = Fp::ONE;
    }
    for (i, x) in [0u64, 3, 5, 17, 30].iter().enumerate() {
        cols[3][i] = Fp::from(*x);
    }
    for r in 5..usable_rows {
        cols[3][r] = Fp::ZERO;
    }
    cols
}

pub fn records(out: &mut Out) -> Vec<u8> {
    let (params, vk) = setup();
    let circuit = RefCircuit;
    let mut cs = ConstraintSystem::<Fp>::default();
    RefCircuit::configure(&mut cs);

    let pinned = format!("{:?}", vk.pinned());
    let quotient = vk.get_domain().get_quotient_poly_degree();
    let extended_k = number_after(&pinned, "extended_k: ");
    assert_eq!(cs.degree(), quotient + 1);
    assert_eq!(extended_k, K as usize + quotient.next_power_of_two().trailing_zeros() as usize);

    let mut hasher = blake2b_simd::Params::new()
        .hash_length(64)
        .personal(b"Halo2-Verify-Key")
        .to_state();
    hasher.update(&(pinned.len() as u64).to_le_bytes());
    hasher.update(pinned.as_bytes());
    let vk_repr = Fp::from_uniform_bytes(hasher.finalize().as_array());

    let fixed_commitments = parse_points(bracket_after(&pinned, "fixed_commitments: ["));
    let permutation_commitments =
        parse_points(bracket_after(&pinned, "permutation: VerifyingKey { commitments: ["));
    let usable_rows = (1usize << K) - (cs.blinding_factors() + 1);
    let cols = fixed_columns(usable_rows);
    let domain = vk.get_domain();
    let recomputed: Vec<EqAffine> = cols
        .iter()
        .map(|c| {
            let poly = domain.lagrange_from_vec(c.clone());
            params.commit_lagrange(&poly, Blind::default()).to_affine()
        })
        .collect();
    assert_eq!(recomputed, fixed_commitments, "recomputed fixed commitments");

    let pk = keygen_pk(&params, vk.clone(), &circuit).expect("keygen_pk");
    let inst = [Fp::from(INSTANCE)];
    let cols_i: [&[Fp]; 1] = [&inst];
    let all: [&[&[Fp]]; 1] = [&cols_i];
    let mut transcript = Blake2bWrite::<Vec<u8>, EqAffine, Challenge255<EqAffine>>::init(vec![]);
    create_proof(&params, &pk, &[circuit.clone()], &[&all[0][..]], OsRng, &mut transcript)
        .expect("create_proof");
    let proof = transcript.finalize();
    let challenges = verify_bytes(&params, &vk, &proof).expect("reference verifier accepts");

    // replay the verifier's first challenge with the independently computed
    // vk representative and instance commitment
    {
        let mut padded = vec![Fp::ZERO; 1 << K];
        padded[0] = Fp::from(INSTANCE);
        let inst_commit = params
            .commit_lagrange(&domain.lagrange_from_vec(padded), Blind::default())
            .to_affine();
        let mut t = Blake2bRead::<&[u8], EqAffine, Challenge255<EqAffine>>::init(&proof[..]);
        t.common_scalar(vk_repr).unwrap();
        t.common_point(inst_commit).unwrap();
        for _ in 0..3 {
            t.read_point().unwrap();
        }
        assert_eq!(*t.squeeze_challenge(), challenges[0], "theta replay");
    }

    // tamper checks
    let mut flipped = proof.clone();
    *flipped.last_mut().unwrap() ^= 1;
    let mut appended = proof.clone();
    appended.push(0);
    let truncated = proof[..proof.len() - 1].to_vec();
    for (name, bad) in [("flipped", flipped), ("appended", appended), ("truncated", truncated)] {
        if verify_bytes(&params, &vk, &bad).is_ok() {
            eprintln!("tamper check failed: {name} proof accepted");
            std::process::exit(1);
        }
    }

    out.rec("circuit_k", vec![K.to_string()]);
    out.rec("circuit_instance", vec![fe(&Fp::from(INSTANCE))]);
    out.rec("circuit_degree", vec![cs.degree().to_string()]);
    out.rec("circuit_blinding_factors", vec![cs.blinding_factors().to_string()]);
    out.rec("circuit_quotient_degree", vec![quotient.to_string()]);
    out.rec("circuit_extended_k", vec![extended_k.to_string()]);
    out.rec("circuit_pinned", vec![hex(pinned.as_bytes())]);
    out.rec("circuit_fixed_commitments", fixed_commitments.iter().map(pt).collect());
    out.rec(
        "circuit_permutation_commitments",
        permutation_commitments.iter().map(pt).collect(),
    );
    out.rec(
        "circuit_fixed_cols",
        cols.iter().flat_map(|c| c.iter().map(fe)).collect(),
    );
    out.rec("circuit_proof_len", vec![proof.len().to_string()]);
    out.rec("circuit_proof", vec![hex(&proof)]);
    out.rec("circuit_challenges", challenges.iter().map(|c| hex(c)).collect());
    proof
}
