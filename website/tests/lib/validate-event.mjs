// Minimal JSON-Schema-subset validator: $ref (#/$defs/*), const, enum, type
// string/array/object, pattern, maxLength, format date-time, required,
// properties, additionalProperties:false, propertyNames.pattern.
export function validate(schema, value, root = schema, path = '$') {
  if (schema.$ref) return validate(root.$defs[schema.$ref.replace('#/$defs/', '')], value, root, path);
  const errors = [];
  if ('const' in schema && value !== schema.const) errors.push(`${path}: expected ${schema.const}`);
  if (schema.enum && !schema.enum.includes(value)) errors.push(`${path}: "${value}" not in enum`);
  if (schema.type === 'string') {
    if (typeof value !== 'string') return [`${path}: expected string`];
    if (schema.pattern && !new RegExp(schema.pattern).test(value)) errors.push(`${path}: "${value}" !~ ${schema.pattern}`);
    if (schema.maxLength && value.length > schema.maxLength) errors.push(`${path}: longer than ${schema.maxLength}`);
    if (schema.format === 'date-time' && !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z$/.test(value)) {
      errors.push(`${path}: "${value}" is not an RFC 3339 UTC time`);
    }
  }
  if (schema.type === 'array') {
    if (!Array.isArray(value)) return [`${path}: expected array`];
    value.forEach((v, i) => errors.push(...validate(schema.items, v, root, `${path}[${i}]`)));
  }
  if (schema.type === 'object') {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return [`${path}: expected object`];
    for (const r of schema.required ?? []) if (!(r in value)) errors.push(`${path}.${r}: required`);
    const props = schema.properties ?? {};
    for (const [k, v] of Object.entries(value)) {
      if (k in props) errors.push(...validate(props[k], v, root, `${path}.${k}`));
      else if (schema.additionalProperties === false) errors.push(`${path}.${k}: not allowed`);
    }
    if (schema.propertyNames?.pattern) {
      const re = new RegExp(schema.propertyNames.pattern);
      for (const k of Object.keys(value)) if (!re.test(k)) errors.push(`${path}.${k}: invalid key`);
    }
  }
  return errors;
}
