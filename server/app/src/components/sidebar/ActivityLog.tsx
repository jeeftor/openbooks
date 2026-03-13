import { Badge, Center, Code, Stack, Text } from "@mantine/core";
import { LogEntry, useGetLogsQuery } from "../../state/api";

const levelColor: Record<LogEntry["level"], string> = {
  info: "brand",
  warn: "yellow",
  error: "red"
};

export default function ActivityLog() {
  const { data, isError } = useGetLogsQuery(null, { pollingInterval: 4_000 });

  if (isError) {
    return (
      <Center>
        <Text color="dimmed" size="sm">
          Log unavailable.
        </Text>
      </Center>
    );
  }

  if (!data || data.length === 0) {
    return (
      <Center>
        <Text color="dimmed" size="sm">
          No activity yet.
        </Text>
      </Center>
    );
  }

  return (
    <Stack spacing={6}>
      {data.map((entry, i) => (
        <ActivityEntry key={i} entry={entry} />
      ))}
    </Stack>
  );
}

function ActivityEntry({ entry }: { entry: LogEntry }) {
  const time = new Date(entry.time).toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  });

  return (
    <Stack spacing={2}>
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <Badge
          size="xs"
          radius="sm"
          variant="light"
          color={levelColor[entry.level]}>
          {entry.level}
        </Badge>
        <Text size="xs" color="dimmed">
          {time}
        </Text>
      </div>
      <Code
        style={{
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
          fontSize: 11
        }}>
        {entry.message}
      </Code>
    </Stack>
  );
}
