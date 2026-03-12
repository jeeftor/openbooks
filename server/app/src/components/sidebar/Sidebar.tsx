import {
  ActionIcon,
  createStyles,
  Group,
  Navbar,
  SegmentedControl,
  Text,
  Tooltip,
  useMantineColorScheme,
  useMantineTheme
} from "@mantine/core";
import { useLocalStorage, useMediaQuery } from "@mantine/hooks";
import {
  BellSimple,
  IdentificationBadge,
  MoonStars,
  Plugs,
  Sidebar as SidebarIcon,
  Sun
} from "phosphor-react";
import { toggleDrawer } from "../../state/notificationSlice";
import { toggleSidebar } from "../../state/stateSlice";
import { useAppDispatch, useAppSelector } from "../../state/store";
import { useGetVersionQuery } from "../../state/api";
import History from "./History";
import Library from "./Library";

const useStyles = createStyles((theme) => {
  return {
    navbar: {
      backgroundColor:
        theme.colorScheme === "dark" ? theme.colors.dark[7] : theme.white
    },
    footer: {
      borderTop: `1px solid ${
        theme.colorScheme === "dark"
          ? theme.colors.dark[4]
          : theme.colors.gray[3]
      }`,
      paddingTop: theme.spacing.sm
    }
  };
});

export function SidebarContent() {
  const { classes } = useStyles();
  const { colorScheme, toggleColorScheme } = useMantineColorScheme();
  const dispatch = useAppDispatch();
  const connected = useAppSelector((store) => store.state.isConnected);
  const username = useAppSelector((store) => store.state.username);
  const { data: version } = useGetVersionQuery(null);

  const [index, setIndex] = useLocalStorage<"books" | "history">({
    key: "sidebar-state",
    defaultValue: "history"
  });

  return (
    <>
      <Navbar.Section p="sm">
        <Group position="apart">
          <Text weight="bold" size="lg">
            OpenBooks
          </Text>
          <Tooltip
            label={`OpenBooks server ${
              connected ? "connected" : "disconnected"
            }.`}>
            <ActionIcon
              disabled={!connected}
              onClick={() => dispatch(toggleDrawer())}>
              <BellSimple weight="bold" size={18} />
            </ActionIcon>
          </Tooltip>
        </Group>

        <Text size="sm" color="dimmed">
          Download eBooks from IRC Highway
        </Text>

        <SegmentedControl
          size="sm"
          styles={(theme) => ({
            root: {
              marginTop: theme.spacing.md
            },
            label: {
              fontSize: theme.fontSizes.xs
            }
          })}
          value={index}
          onChange={(value: "books" | "history") => setIndex(value)}
          data={[
            { label: "Search History", value: "history" },
            { label: "Previous Downloads", value: "books" }
          ]}
          fullWidth
        />
      </Navbar.Section>

      <Navbar.Section grow p="xs" style={{ overflow: "auto" }}>
        {index === "history" ? <History /> : <Library />}
      </Navbar.Section>

      <Navbar.Section className={classes.footer} p="sm">
        <Group position="apart" noWrap>
          <Group>
            {username ? (
              <>
                <IdentificationBadge size={24} />
                <Text
                  size="sm"
                  lineClamp={1}
                  style={{
                    maxWidth: 150,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap"
                  }}>
                  {username}
                </Text>
              </>
            ) : (
              <>
                <Plugs size={24} />
                <Text size="sm">Not connected.</Text>
              </>
            )}
          </Group>

          <Group align="end" spacing="xs">
            {version && (
              <Text size="xs" color="dimmed">
                {version}
              </Text>
            )}
            <ActionIcon onClick={() => toggleColorScheme()}>
              {colorScheme === "dark" ? (
                <Sun size={18} weight="bold" />
              ) : (
                <MoonStars size={18} weight="bold" />
              )}
            </ActionIcon>
            <ActionIcon onClick={() => dispatch(toggleSidebar())}>
              <SidebarIcon weight="bold" size={18} />
            </ActionIcon>
          </Group>
        </Group>
      </Navbar.Section>
    </>
  );
}

export default function Sidebar() {
  const { classes } = useStyles();
  const theme = useMantineTheme();
  const isMobile = useMediaQuery("(max-width: 768px)");
  const opened = useAppSelector((store) => store.state.isSidebarOpen);

  if (!opened || isMobile) {
    return <></>;
  }

  return (
    <Navbar
      width={{ sm: 300 }}
      hiddenBreakpoint="sm"
      className={classes.navbar}>
      <SidebarContent />
    </Navbar>
  );
}
