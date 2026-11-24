use halo2_proofs::circuit::{AssignedCell, Layouter, SimpleFloorPlanner, Value};
use halo2_proofs::pasta::{EqAffine, Fp};
use halo2_proofs::plonk::{
    create_proof, keygen_pk, keygen_vk, verify_proof, Advice, Circuit, Column, ConstraintSystem,
    Error, Fixed, Instance, Selector, SingleVerifier, TableColumn, VerifyingKey,
};
use halo2_proofs::poly::commitment::Params;
use halo2_proofs::poly::Rotation;
use halo2_proofs::transcript::{Blake2bRead, Blake2bWrite, Challenge255};
use rand_core::OsRng;

use crate::util::*;

pub const K: u32 = 4;
pub const WITNESSES: [(u64, u64); 2] = [(3, 4), (5, 6)];
const TABLE: u64 = 8;
const OFFSET: u64 = 5;
const SHIFT: u64 = 7;

/// The public inputs of the circuit for the witness `(x, y)`: `(x * y + 5)^2` and that plus 7.
pub fn instance(x: u64, y: u64) -> Vec<Fp> {
    let square = (x * y + OFFSET) * (x * y + OFFSET);
    vec![Fp::from(square), Fp::from(square + SHIFT)]
}

#[derive(Clone, Debug)]
pub struct ArithConfig {
    a: Column<Advice>,
    b: Column<Advice>,
    c: Column<Advice>,
    d: Column<Advice>,
    e: Column<Advice>,
    inst: Column<Instance>,
    s_mul: Selector,
    s_add: Selector,
    s_square: Selector,
    q_range: Selector,
    table: TableColumn,
}

/// A chip with a multiplication, an addition and a squaring gate, each behind a
/// simple selector, and a range lookup behind a complex selector.
struct ArithChip {
    config: ArithConfig,
}

impl ArithChip {
    fn configure(meta: &mut ConstraintSystem<Fp>) -> ArithConfig {
        let constants: Column<Fixed> = meta.fixed_column();
        let table = meta.lookup_table_column();
        let a = meta.advice_column();
        let b = meta.advice_column();
        let c = meta.advice_column();
        let d = meta.advice_column();
        let e = meta.advice_column();
        let inst = meta.instance_column();
        meta.enable_constant(constants);
        for column in [a, b, c, d, e] {
            meta.enable_equality(column);
        }
        meta.enable_equality(inst);
        let s_mul = meta.selector();
        let s_add = meta.selector();
        let s_square = meta.selector();
        let q_range = meta.complex_selector();

        meta.create_gate("mul", |m| {
            let s = m.query_selector(s_mul);
            let a = m.query_advice(a, Rotation::cur());
            let b = m.query_advice(b, Rotation::cur());
            let c = m.query_advice(c, Rotation::cur());
            vec![s * (a * b - c)]
        });
        meta.create_gate("add", |m| {
            let s = m.query_selector(s_add);
            let a = m.query_advice(a, Rotation::cur());
            let b = m.query_advice(b, Rotation::cur());
            let c = m.query_advice(c, Rotation::cur());
            vec![s * (a + b - c)]
        });
        meta.create_gate("square", |m| {
            let s = m.query_selector(s_square);
            let d = m.query_advice(d, Rotation::cur());
            let e = m.query_advice(e, Rotation::cur());
            vec![s * (d.clone() * d - e)]
        });
        meta.lookup(|m| {
            let q = m.query_selector(q_range);
            let a = m.query_advice(a, Rotation::cur());
            vec![(q * a, table)]
        });

        ArithConfig { a, b, c, d, e, inst, s_mul, s_add, s_square, q_range, table }
    }

    fn load_table(&self, layouter: &mut impl Layouter<Fp>) -> Result<(), Error> {
        layouter.assign_table(
            || "range",
            |mut table| {
                for i in 0..TABLE {
                    table.assign_cell(|| "", self.config.table, i as usize, || {
                        Value::known(Fp::from(i))
                    })?;
                }
                Ok(())
            },
        )
    }

    fn mul(
        &self,
        layouter: &mut impl Layouter<Fp>,
        x: Value<Fp>,
        y: Value<Fp>,
    ) -> Result<AssignedCell<Fp, Fp>, Error> {
        let cfg = &self.config;
        layouter.assign_region(
            || "mul",
            |mut region| {
                cfg.s_mul.enable(&mut region, 0)?;
                cfg.q_range.enable(&mut region, 0)?;
                region.assign_advice(|| "x", cfg.a, 0, || x)?;
                region.assign_advice(|| "y", cfg.b, 0, || y)?;
                region.assign_advice(|| "x * y", cfg.c, 0, || x * y)
            },
        )
    }

    fn add_constant(
        &self,
        layouter: &mut impl Layouter<Fp>,
        input: &AssignedCell<Fp, Fp>,
        constant: u64,
    ) -> Result<AssignedCell<Fp, Fp>, Error> {
        let cfg = &self.config;
        layouter.assign_region(
            || "add constant",
            |mut region| {
                cfg.s_add.enable(&mut region, 0)?;
                let a = input.copy_advice(|| "input", &mut region, cfg.a, 0)?;
                let b = region.assign_advice_from_constant(|| "constant", cfg.b, 0, Fp::from(constant))?;
                region.assign_advice(|| "sum", cfg.c, 0, || {
                    a.value().copied() + b.value().copied()
                })
            },
        )
    }

    fn square(
        &self,
        layouter: &mut impl Layouter<Fp>,
        input: &AssignedCell<Fp, Fp>,
    ) -> Result<AssignedCell<Fp, Fp>, Error> {
        let cfg = &self.config;
        layouter.assign_region(
            || "square",
            |mut region| {
                cfg.s_square.enable(&mut region, 0)?;
                let d = input.copy_advice(|| "input", &mut region, cfg.d, 0)?;
                region.assign_advice(|| "square", cfg.e, 0, || {
                    d.value().copied() * d.value().copied()
                })
            },
        )
    }
}

#[derive(Clone, Default)]
pub struct RegionCircuit {
    pub x: Value<Fp>,
    pub y: Value<Fp>,
}

impl Circuit<Fp> for RegionCircuit {
    type Config = ArithConfig;
    type FloorPlanner = SimpleFloorPlanner;

    fn without_witnesses(&self) -> Self {
        Self::default()
    }

    fn configure(meta: &mut ConstraintSystem<Fp>) -> ArithConfig {
        ArithChip::configure(meta)
    }

    fn synthesize(&self, config: ArithConfig, mut layouter: impl Layouter<Fp>) -> Result<(), Error> {
        let chip = ArithChip { config: config.clone() };
        chip.load_table(&mut layouter)?;
        let product = chip.mul(&mut layouter, self.x, self.y)?;
        let sum = chip.add_constant(&mut layouter, &product, OFFSET)?;
        let square = chip.square(&mut layouter, &sum)?;
        let shifted = chip.add_constant(&mut layouter, &square, SHIFT)?;
        layouter.constrain_instance(square.cell(), config.inst, 0)?;
        layouter.constrain_instance(shifted.cell(), config.inst, 1)
    }
}

pub fn setup() -> (Params<EqAffine>, VerifyingKey<EqAffine>) {
    let params = Params::<EqAffine>::new(K);
    let vk = keygen_vk(&params, &RegionCircuit::default()).expect("keygen_vk");
    (params, vk)
}

fn instances() -> Vec<Vec<Fp>> {
    WITNESSES.iter().map(|&(x, y)| instance(x, y)).collect()
}

/// Verifies a proof of the two circuits of `WITNESSES` and rejects bytes left over.
pub fn verify_bytes(
    params: &Params<EqAffine>,
    vk: &VerifyingKey<EqAffine>,
    proof: &[u8],
) -> Result<(), String> {
    let values = instances();
    let columns: Vec<[&[Fp]; 1]> = values.iter().map(|v| [&v[..]]).collect();
    let all: Vec<&[&[Fp]]> = columns.iter().map(|c| &c[..]).collect();
    let mut rest = proof;
    {
        let mut t = Blake2bRead::<&mut &[u8], EqAffine, Challenge255<EqAffine>>::init(&mut rest);
        verify_proof(params, vk, SingleVerifier::new(params), &all, &mut t)
            .map_err(|e| format!("{e}").replace('\n', " "))?;
    }
    if !rest.is_empty() {
        return Err(format!("{} trailing bytes after proof", rest.len()));
    }
    Ok(())
}

pub fn records(out: &mut Out) {
    let (params, vk) = setup();
    let pinned = format!("{:?}", vk.pinned());
    let pk = keygen_pk(&params, vk.clone(), &RegionCircuit::default()).expect("keygen_pk");
    let circuits: Vec<RegionCircuit> = WITNESSES
        .iter()
        .map(|&(x, y)| RegionCircuit { x: Value::known(Fp::from(x)), y: Value::known(Fp::from(y)) })
        .collect();
    let values = instances();
    let columns: Vec<[&[Fp]; 1]> = values.iter().map(|v| [&v[..]]).collect();
    let all: Vec<&[&[Fp]]> = columns.iter().map(|c| &c[..]).collect();
    let mut transcript = Blake2bWrite::<Vec<u8>, EqAffine, Challenge255<EqAffine>>::init(vec![]);
    create_proof(&params, &pk, &circuits, &all, OsRng, &mut transcript).expect("create_proof");
    let proof = transcript.finalize();
    verify_bytes(&params, &vk, &proof).expect("reference verifier accepts");
    let mut flipped = proof.clone();
    flipped[proof.len() / 2] ^= 1;
    if verify_bytes(&params, &vk, &flipped).is_ok() {
        eprintln!("tamper check failed: flipped regions proof accepted");
        std::process::exit(1);
    }
    let fixed_commitments = pinned_points(&pinned, "fixed_commitments: [");
    let permutation_commitments = pinned_points(&pinned, "permutation: VerifyingKey { commitments: [");
    out.rec("regions_k", vec![K.to_string()]);
    out.rec(
        "regions_instances",
        values.iter().flat_map(|v| v.iter().map(fe)).collect(),
    );
    out.rec("regions_pinned", vec![hex(pinned.as_bytes())]);
    out.rec("regions_fixed_commitments", fixed_commitments);
    out.rec("regions_permutation_commitments", permutation_commitments);
    out.rec("regions_proof", vec![hex(&proof)]);
}

fn pinned_points(pinned: &str, marker: &str) -> Vec<String> {
    crate::circuit::parse_points(crate::circuit::bracket_after(pinned, marker))
        .iter()
        .map(pt)
        .collect()
}
