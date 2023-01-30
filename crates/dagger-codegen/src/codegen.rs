use std::{
    io::{BufWriter, Write},
    sync::Arc,
};

use dagger_core::introspection::{FullType, IntrospectionResponse, Schema};
use genco::{fmt, prelude::rust, prelude::*, quote};

use crate::handlers::{
    enumeration::Enumeration, input::Input, object::Object, scalar::Scalar, DynHandler, Handlers,
};

#[allow(dead_code)]
pub struct CodeGeneration {
    handlers: Handlers,
}

impl CodeGeneration {
    pub fn new() -> Self {
        Self {
            handlers: vec![
                Arc::new(Scalar {}),
                Arc::new(Enumeration {}),
                Arc::new(Input {}),
                Arc::new(Object {}),
            ],
        }
    }

    pub fn generate(&self, schema: &IntrospectionResponse) -> eyre::Result<String> {
        let code = self.generate_from_schema(
            schema
                .as_schema()
                .schema
                .as_ref()
                .ok_or(eyre::anyhow!("could not get schema to generate code from"))?,
        )?;
        Ok(code)
    }

    fn generate_from_schema(&self, schema: &Schema) -> eyre::Result<String> {
        let mut output = rust::Tokens::new();
        output.push();
        output.append(quote! {
            $(format!("// code generated by dagger. DO NOT EDIT."))
        });

        output.push();
        output.append(render_base_types());
        output.push();

        let types = get_types(schema)?;
        //let remaining: Vec<Option<String>> = types.into_iter().map(type_name).collect();
        //
        for (handler, types) in self.group_by_handlers(&types) {
            for t in types {
                if let Some(_) = self.type_name(&t) {
                    let rendered = handler.render(&t)?;
                    output.push();
                    output.append(rendered);
                }
            }
        }

        let mut buffer = BufWriter::new(Vec::new());
        let mut w = fmt::IoWriter::new(buffer.by_ref());
        let fmt = fmt::Config::from_lang::<Rust>().with_indentation(fmt::Indentation::Space(4));
        let config = rust::Config::default();
        // Prettier imports and use.
        //.with_default_import(rust::ImportMode::Qualified);

        output.format_file(&mut w.as_formatter(&fmt), &config)?;

        let out = String::from_utf8(buffer.into_inner()?)?;
        Ok(out)
    }

    pub fn group_by_handlers(&self, types: &Vec<&FullType>) -> Vec<(DynHandler, Vec<FullType>)> {
        let mut group = vec![];

        for handler in self.handlers.iter() {
            let mut group_types: Vec<FullType> = vec![];
            for t in types.iter() {
                if handler.predicate(*t) {
                    group_types.push(t.clone().clone());
                }
            }

            group.push((handler.clone(), group_types))
        }

        group
    }

    pub fn type_name(&self, t: &FullType) -> Option<String> {
        let name = t.name.as_ref();
        if let Some(name) = name {
            if name.starts_with("_") {
                //|| !is_custom_scalar_type(t) {
                return None;
            }

            return Some(name.replace("Query", "Client"));
        }

        None
    }

    fn group_key(&self, t: &FullType) -> Option<DynHandler> {
        for handler in self.handlers.iter() {
            if handler.predicate(&t) {
                return Some(handler.clone());
            }
        }

        None
    }

    fn sort_key(&self, t: &FullType) -> (isize, String) {
        for (i, handler) in self.handlers.iter().enumerate() {
            if handler.predicate(t) {
                return (i as isize, t.name.as_ref().unwrap().clone());
            }
        }

        return (-1, t.name.as_ref().unwrap().clone());
    }
}

fn render_base_types() -> rust::Tokens {
    let i = rust::import("dagger_core", "Int");
    let b = rust::import("dagger_core", "Boolean");

    quote! {
        $(register(i))
        $(register(b))
    }
}

fn get_types(schema: &Schema) -> eyre::Result<Vec<&FullType>> {
    let types = schema
        .types
        .as_ref()
        .ok_or(eyre::anyhow!("types not found on schema"))?;

    let types: Vec<&FullType> = types
        .iter()
        .map(|t| t.as_ref().map(|t| &t.full_type))
        .flatten()
        .collect();

    Ok(types)
}
