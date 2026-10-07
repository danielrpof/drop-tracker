// Prints a TS/TSX file reprinted by the TypeScript compiler with comments
// removed, so two versions that differ only in comments (or formatting) print
// identically. Empty JSX expression containers (what a deleted {/* */} leaves)
// are dropped too. Run from the repo root: node strip-ts.cjs <file|-> <name>
const fs = require("fs")
const path = require("path")
const ts = require(path.resolve("web/node_modules/typescript"))

const [, , file, name = file] = process.argv
const src = fs.readFileSync(file === "-" ? 0 : file, "utf8")
const kind = name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS
const sf = ts.createSourceFile(name, src, ts.ScriptTarget.Latest, false, kind)

const dropEmptyJsx = (ctx) => (root) => {
  const visit = (node) => {
    if ((ts.isJsxElement(node) || ts.isJsxFragment(node)) && node.children) {
      const kids = node.children.filter(
        (c) => !(ts.isJsxExpression(c) && !c.expression) && !(ts.isJsxText(c) && c.containsOnlyTriviaWhiteSpaces)
      )
      node = ts.isJsxElement(node)
        ? ctx.factory.updateJsxElement(node, node.openingElement, kids, node.closingElement)
        : ctx.factory.updateJsxFragment(node, node.openingFragment, kids, node.closingFragment)
    }
    return ts.visitEachChild(node, visit, ctx)
  }
  return ts.visitNode(root, visit)
}

const out = ts.transform(sf, [dropEmptyJsx]).transformed[0]
process.stdout.write(ts.createPrinter({ removeComments: true }).printFile(out))
