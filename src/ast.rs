#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Module {
    pub items: Vec<Item>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Item {
    Fn(FnDecl),
    Workflow(WorkflowDecl),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FnDecl {
    pub name: String,
    pub params: Vec<Param>,
    pub return_type: Option<TypeRef>,
    pub effects: Option<EffectsDecl>,
    pub contracts: Vec<Contract>,
    pub body: Block,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkflowDecl {
    pub name: String,
    pub params: Vec<Param>,
    pub return_type: Option<TypeRef>,
    pub effects: Option<EffectsDecl>,
    pub contracts: Vec<Contract>,
    pub body: Block,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Param {
    pub name: String,
    pub ty: Option<TypeRef>,
}

pub type Path = Vec<String>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TypeRef {
    Path(Path),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EffectsDecl {
    Effects(Vec<Path>),
    Requires(Vec<(String, String)>),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContractKind {
    Pre,
    Post,
    Invariant,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Contract {
    pub kind: ContractKind,
    pub expr: Expr,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Block {
    pub stmts: Vec<Stmt>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Stmt {
    Let(LetStmt),
    Return(ReturnStmt),
    If(IfStmt),
    Loop(LoopStmt),
    Step(StepStmt),
    Expr(Expr),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LetStmt {
    pub name: String,
    pub ty: Option<TypeRef>,
    pub expr: Expr,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReturnStmt {
    pub expr: Option<Expr>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IfStmt {
    pub condition: Expr,
    pub then_block: Block,
    pub else_block: Option<Block>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LoopStmt {
    pub body: Block,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StepStmt {
    pub label: String,
    pub body: Block,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Expr {
    Ident(String),
    String(String),
    Path(Path),
    Call { callee: Path, args: Vec<Expr> },
}
