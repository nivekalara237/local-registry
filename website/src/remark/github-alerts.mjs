// Turns GitHub alerts ("> [!NOTE]" blockquotes) into Docusaurus admonitions, so a page reads
// well both on GitHub and on the site. Runs before the admonitions plugin.
const kinds = {NOTE: 'note', TIP: 'tip', IMPORTANT: 'info', WARNING: 'warning', CAUTION: 'danger'};

export default function githubAlerts() {
  const visit = (node) => {
    if (!node.children) return;
    node.children = node.children.map((child) => {
      visit(child);
      if (child.type !== 'blockquote') return child;
      const first = child.children[0];
      const text = first?.type === 'paragraph' ? first.children[0] : undefined;
      const m = text?.type === 'text' && /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*\n?/.exec(text.value);
      if (!m) return child;
      text.value = text.value.slice(m[0].length);
      if (text.value === '') first.children.shift();
      if (first.children.length === 0) child.children.shift();
      return {type: 'containerDirective', name: kinds[m[1]], attributes: {}, children: child.children};
    });
  };
  return visit;
}
