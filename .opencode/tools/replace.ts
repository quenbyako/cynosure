import { tool } from "@opencode-ai/plugin";
import * as fs from "fs";
import * as path from "path";

// Парсер SEARCH/REPLACE блоков с нормализацией переносов строк
export function parseSearchReplaceBlocks(edits: string): Array<{ search: string; replace: string }> {
  const blocks: Array<{ search: string; replace: string }> = [];
  const normalized = edits.replace(/\r\n/g, "\n");

  const searchMarker = "<<<<<<< SEARCH\n";
  const dividerMarker = "\n=======\n";
  const replaceMarker = "\n>>>>>>> REPLACE";

  let cursor = 0;
  while (true) {
    const startIndex = normalized.indexOf(searchMarker, cursor);
    if (startIndex === -1) break;

    const divIndex = normalized.indexOf(dividerMarker, startIndex);
    if (divIndex === -1) {
      throw new Error("Неверный формат блока: отсутствует разделитель =======");
    }

    const endIndex = normalized.indexOf(replaceMarker, divIndex);
    if (endIndex === -1) {
      throw new Error("Неверный формат блока: отсутствует закрывающий маркер >>>>>>> REPLACE");
    }

    const searchContent = normalized.substring(startIndex + searchMarker.length, divIndex);
    const replaceContent = normalized.substring(divIndex + dividerMarker.length, endIndex);

    blocks.push({ search: searchContent, replace: replaceContent });
    cursor = endIndex + replaceMarker.length;
  }

  if (blocks.length === 0) {
    throw new Error("Блоки SEARCH/REPLACE не найдены. Убедитесь, что используете правильные маркеры.");
  }

  return blocks;
}

export default tool({
  description: "Apply one or more SEARCH/REPLACE blocks to a file. Extremely robust against formatting and line numbering issues.",
  args: {
    filePath: tool.schema.string().describe("Relative or absolute path to the file to modify"),
    edits: tool.schema.string().describe(
      "One or more blocks of edits using the format:\n" +
      "<<<<<<< SEARCH\nfunc processMessage(msg string) error {\n    log.Print(\"processing\")\n    return nil\n}\n=======\nfunc processMessage(msg string) error {\n    log.Printf(\"processing message: %s\", msg)\n    return nil\n}\n>>>>>>> REPLACE"
    ),
  },
  async execute({ filePath, edits }, context) {
    const absolutePath = path.isAbsolute(filePath)
      ? filePath
      : path.join(context.directory, filePath);

    if (!fs.existsSync(absolutePath)) {
      return {
        output: `Error: file not found at ${filePath}`,
        metadata: { success: false },
      };
    }

    let fileContent = fs.readFileSync(absolutePath, "utf-8").replace(/\r\n/g, "\n");
    let blocks: Array<{ search: string; replace: string }>;

    try {
      blocks = parseSearchReplaceBlocks(edits);
    } catch (err: any) {
      return {
        output: `Ошибка парсинга блоков: ${err.message}`,
        metadata: { success: false },
      };
    }

    for (const [index, block] of blocks.entries()) {
      const { search, replace } = block;

      if (search === "") {
        fileContent = replace + "\n" + fileContent;
        continue;
      }

      const occurrences = fileContent.split(search).length - 1;

      if (occurrences === 0) {
        return {
          output: `Ошибка в блоке #${index + 1}: Код из блока SEARCH не найден в файле. Проверьте форматирование и пробелы.\n\nИскомый код:\n${search}`,
          metadata: { success: false },
        };
      }

      if (occurrences > 1) {
        return {
          output: `Ошибка в блоке #${index + 1}: Код из блока SEARCH найден ${occurrences} раз(а). Сделайте блок поиска более уникальным.`,
          metadata: { success: false },
        };
      }

      fileContent = fileContent.replace(search, replace);
    }

    fs.writeFileSync(absolutePath, fileContent, "utf-8");

    return {
      output: `Изменения успешно применены к файлу ${filePath}. Применено блоков: ${blocks.length}.`,
      metadata: { success: true },
    };
  },
});
