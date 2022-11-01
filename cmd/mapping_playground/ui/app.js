async function mappingPlayground(template, values) {
  const resp = await fetch("/mapping", {
    body: JSON.stringify({ template, values }),
    method: "POST",
  })

  return await resp.json()
}

const initialMappingValues = {
  Kind: "tenant",
  Tenant: { Name: "tenant1" },
  Management: {},
  Env: { name: "env1", kind: "tenant" },
  Envs: [{ name: "env2", kind: "tenant" }],
}

const initialTemplate = `
{{ .Envs | toJSON }}
`

let initalTemplate = localStorage.getItem("template") || initialTemplate.trim()
let initialValues = localStorage.getItem("values") || JSON.stringify(initialMappingValues, null, 2)

const tpl = document.getElementById("template")
const vals = document.getElementById("values")
const out = document.getElementById("output")

const tplFlask = new CodeFlask(tpl, { language: 'text', defaultTheme: false });
const valsFlask = new CodeFlask(vals, { language: 'json', defaultTheme: false });


tplFlask.updateCode(initalTemplate)
valsFlask.updateCode(initialValues)

const update = async () => {
  const template = tplFlask.getCode()
  const values = valsFlask.getCode()
  if (!template) return;

  try {
    var valString = JSON.parse(values)
  } catch (e) {
    out.innerText = "Invalid JSON: " + e
    return
  }



  const result = JSON.stringify(await mappingPlayground(template, valString), null, 2)
  out.innerText = result
  localStorage.setItem("template", template)
  localStorage.setItem("values", values)
}

let timeout = null
const onChange = () => {
  clearTimeout(timeout)
  timeout = setTimeout(update, 100)
}

tplFlask.onUpdate(onChange)
valsFlask.onUpdate(onChange)

update()
