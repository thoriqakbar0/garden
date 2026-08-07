import { defineBashTool, defineTool } from "eve/tools";
import { never } from "eve/tools/approval";

export default defineTool({
  ...defineBashTool(),
  approval: never(),
});
