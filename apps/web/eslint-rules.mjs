const visibleAttributes = new Set([
  "placeholder",
  "title",
  "alt",
  "aria-label",
  "label",
]);
const literalText = (node) =>
  node?.type === "Literal" &&
  typeof node.value === "string" &&
  node.value.trim().length > 0;
export const i18nText = {
  meta: {
    type: "problem",
    schema: [],
    messages: {
      resource:
        "Move user-visible copy into an i18next resource and read it with t().",
    },
  },
  create(context) {
    return {
      JSXText(node) {
        if (node.value.trim()) context.report({ node, messageId: "resource" });
      },
      JSXAttribute(node) {
        if (!visibleAttributes.has(node.name.name)) return;
        if (
          literalText(node.value) ||
          (node.value?.type === "JSXExpressionContainer" &&
            literalText(node.value.expression))
        ) {
          context.report({ node, messageId: "resource" });
        }
      },
      JSXExpressionContainer(node) {
        if (node.parent.type !== "JSXAttribute" && literalText(node.expression))
          context.report({ node, messageId: "resource" });
      },
    };
  },
};
export const sdkOnly = [
  "error",
  {
    selector: "CallExpression[callee.name='fetch']",
    message: "Use the generated API SDK and the shared client.",
  },
  {
    selector:
      "CallExpression[callee.type='MemberExpression'][callee.property.name='fetch']",
    message: "Use the generated API SDK and the shared client.",
  },
];
