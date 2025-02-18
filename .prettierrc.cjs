/** @type {import('prettier').Options} */
module.exports = {
	useTabs: true,
	singleQuote: false,
	trailingComma: "all",
	printWidth: 100,
	plugins: [require.resolve("prettier-plugin-sql-custom")],
	overrides: [
		{
			files: "*.sql",
			options: {
				language: "postgresql",
				paramTypes: "{ named: ['@'], custom: [{ regex: '\\w+\\.\\w+' }] }",
				functionCase: "upper",
				keywordCase: "upper",
				dataTypeCase: "upper",
				newlineBeforeSemicolon: true,
			},
		},
	],
};
