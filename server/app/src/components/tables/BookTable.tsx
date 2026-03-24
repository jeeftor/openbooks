import {
  ActionIcon,
  Badge,
  Box,
  Button,
  Card,
  Chip,
  Drawer,
  Group,
  Indicator,
  Loader,
  ScrollArea,
  Table,
  Text,
  TextInput,
  Tooltip,
  useMantineTheme
} from "@mantine/core";
import { useElementSize, useMediaQuery, useMergedRef } from "@mantine/hooks";
import {
  createColumnHelper,
  FilterFn,
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  Row,
  useReactTable
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  DownloadSimple,
  Funnel,
  MagnifyingGlass,
  User
} from "phosphor-react";
import { useMemo, useRef, useState } from "react";
import { useSelector } from "react-redux";
import { useGetServersQuery } from "../../state/api";
import { BookDetail } from "../../state/messages";
import { sendDownload } from "../../state/stateSlice";
import { RootState, useAppDispatch } from "../../state/store";
import FacetFilter, {
  ServerFacetEntry,
  StandardFacetEntry
} from "./Filters/FacetFilter";
import { TextFilter } from "./Filters/TextFilter";
import { useTableStyles } from "./styles";

const columnHelper = createColumnHelper<BookDetail>();

const stringInArray: FilterFn<any> = (
  row,
  columnId: string,
  filterValue: string[] | undefined
) => {
  if (!filterValue || filterValue.length === 0) return true;
  return filterValue.includes(row.getValue<string>(columnId));
};

interface BookTableProps {
  books: BookDetail[];
}

export default function BookTable({ books }: BookTableProps) {
  const { classes, cx, theme } = useTableStyles();
  const { data: servers } = useGetServersQuery(null, { pollingInterval: 30000 });

  const { ref: elementSizeRef, width } = useElementSize();
  const virtualizerRef = useRef<HTMLDivElement>();
  const mergedRef = useMergedRef(elementSizeRef, virtualizerRef);

  const isMobile = useMediaQuery("(max-width: 768px)");

  // Mobile filter state
  const [authorFilter, setAuthorFilter] = useState("");
  const [titleFilter, setTitleFilter] = useState("");
  const [formatFilter, setFormatFilter] = useState("");
  const [filterOpen, setFilterOpen] = useState(false);

  const sortedBooks = useMemo(() => {
    if (!servers?.length) return books;
    return [...books].sort((a, b) => {
      const aOnline = servers.includes(a.server) ? 0 : 1;
      const bOnline = servers.includes(b.server) ? 0 : 1;
      return aOnline - bOnline;
    });
  }, [books, servers]);

  const formats = useMemo(
    () => [...new Set(sortedBooks.map((b) => b.format))].filter(Boolean).sort(),
    [sortedBooks]
  );

  const filteredBooks = useMemo(() => {
    let result = sortedBooks;
    if (authorFilter)
      result = result.filter((b) =>
        b.author.toLowerCase().includes(authorFilter.toLowerCase())
      );
    if (titleFilter)
      result = result.filter((b) =>
        b.title.toLowerCase().includes(titleFilter.toLowerCase())
      );
    if (formatFilter) result = result.filter((b) => b.format === formatFilter);
    return result;
  }, [books, authorFilter, titleFilter, formatFilter]);

  // Desktop table columns
  const columns = useMemo(() => {
    const cols = (n: number) => (width / 12) * n;
    return [
      columnHelper.accessor("server", {
        header: (props) => (
          <FacetFilter
            placeholder="Server"
            column={props.column}
            table={props.table}
            Entry={ServerFacetEntry}
          />
        ),
        cell: (props) => {
          const online = servers?.includes(props.getValue());
          return (
            <Text size={12} weight="normal" color="dark" style={{ marginLeft: 20 }}>
              <Tooltip position="top-start" label={online ? "Online" : "Offline"}>
                <Indicator
                  zIndex={0}
                  position="middle-start"
                  offset={-16}
                  size={6}
                  color={online ? "green.6" : "gray"}>
                  {props.getValue()}
                </Indicator>
              </Tooltip>
            </Text>
          );
        },
        size: cols(1),
        enableColumnFilter: true,
        filterFn: stringInArray
      }),
      columnHelper.accessor("author", {
        header: (props) => (
          <TextFilter
            icon={<User weight="bold" />}
            placeholder="Author"
            column={props.column}
            table={props.table}
          />
        ),
        size: cols(2),
        enableColumnFilter: false
      }),
      columnHelper.accessor("title", {
        header: (props) => (
          <TextFilter
            icon={<MagnifyingGlass weight="bold" />}
            placeholder="Title"
            column={props.column}
            table={props.table}
          />
        ),
        minSize: 20,
        size: cols(6),
        enableColumnFilter: false
      }),
      columnHelper.accessor("format", {
        header: (props) => (
          <FacetFilter
            placeholder="Format"
            column={props.column}
            table={props.table}
            Entry={StandardFacetEntry}
          />
        ),
        size: cols(1),
        enableColumnFilter: false,
        filterFn: stringInArray
      }),
      columnHelper.accessor("size", {
        header: "Size",
        size: cols(1),
        enableColumnFilter: false
      }),
      columnHelper.display({
        header: "Download",
        size: cols(1),
        enableColumnFilter: false,
        cell: ({ row }) => (
          <DownloadButton
            book={row.original.full}
            author={row.original.author}
            title={row.original.title}
          />
        )
      })
    ];
  }, [width, servers]);

  const table = useReactTable({
    data: sortedBooks,
    columns,
    enableFilters: true,
    columnResizeMode: "onChange",
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues()
  });

  const { rows: tableRows } = table.getRowModel();

  const rowVirtualizer = useVirtualizer({
    count: tableRows.length,
    getScrollElement: () => virtualizerRef.current ?? null,
    estimateSize: () => 50,
    overscan: 10
  });

  const mobileVirtualizer = useVirtualizer({
    count: filteredBooks.length,
    getScrollElement: () => virtualizerRef.current ?? null,
    estimateSize: () => 82,
    overscan: 8
  });

  // ── Mobile: compact vertical card list ──────────────────────────────────
  if (isMobile) {
    const activeFilterCount = [authorFilter, titleFilter, formatFilter].filter(
      Boolean
    ).length;

    return (
      <Box
        className={classes.container}
        style={{ display: "flex", flexDirection: "column" }}>
        {/* Header: result count + filter button */}
        <Group
          px="sm"
          py={6}
          position="apart"
          style={{
            flexShrink: 0,
            borderBottom: `1px solid ${
              theme.colorScheme === "dark"
                ? theme.colors.dark[4]
                : theme.colors.gray[2]
            }`
          }}>
          <Text size="xs" color="dimmed">
            {filteredBooks.length} of {books.length} results
          </Text>
          <ActionIcon
            variant={activeFilterCount > 0 ? "filled" : "light"}
            onClick={() => setFilterOpen(true)}
            size="md">
            <Funnel
              weight={activeFilterCount > 0 ? "fill" : "bold"}
              size={16}
            />
          </ActionIcon>
        </Group>

        {/* Virtualized card list */}
        <ScrollArea
          viewportRef={mergedRef}
          style={{ flex: 1 }}
          type="hover"
          scrollbarSize={4}
          offsetScrollbars={false}>
          <div
            style={{
              height: mobileVirtualizer.getTotalSize(),
              position: "relative",
              padding: `${theme.spacing.xs}px`
            }}>
            {mobileVirtualizer.getVirtualItems().map((vItem) => {
              const book = filteredBooks[vItem.index];
              return (
                <div
                  key={vItem.key}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: theme.spacing.xs,
                    right: theme.spacing.xs,
                    transform: `translateY(${vItem.start}px)`,
                    paddingBottom: theme.spacing.xs
                  }}>
                  <Card withBorder radius="sm" p="xs">
                    <Group noWrap position="apart" align="flex-start">
                      <Box style={{ flex: 1, minWidth: 0 }}>
                        <Text weight={600} size="sm" lineClamp={1}>
                          {book.title}
                        </Text>
                        <Text color="dimmed" size="xs" lineClamp={1}>
                          {book.author}
                        </Text>
                        <Group mt={4} spacing={6}>
                          {book.format && (
                            <Badge size="xs" variant="light" color="brand">
                              {book.format.toUpperCase()}
                            </Badge>
                          )}
                          {book.size && (
                            <Text size="xs" color="dimmed">
                              {book.size}
                            </Text>
                          )}
                        </Group>
                      </Box>
                      <CardDownloadButton
                        book={book.full}
                        author={book.author}
                        title={book.title}
                      />
                    </Group>
                  </Card>
                </div>
              );
            })}
          </div>
        </ScrollArea>

        {/* Filter drawer */}
        <Drawer
          opened={filterOpen}
          onClose={() => setFilterOpen(false)}
          position="bottom"
          size="auto"
          withCloseButton
          padding="md"
          title="Filter Results">
          <TextInput
            label="Author"
            placeholder="Filter by author"
            value={authorFilter}
            onChange={(e) => setAuthorFilter(e.target.value)}
            icon={<User weight="bold" size={14} />}
          />
          <TextInput
            label="Title"
            placeholder="Filter by title"
            value={titleFilter}
            onChange={(e) => setTitleFilter(e.target.value)}
            icon={<MagnifyingGlass weight="bold" size={14} />}
            mt="sm"
          />
          <Text size="sm" weight={500} mt="sm" mb={4}>
            Format
          </Text>
          <Chip.Group
            value={formatFilter}
            onChange={(v) => setFormatFilter(v as string)}>
            <Group spacing="xs">
              <Chip value="" size="sm">
                All
              </Chip>
              {formats.map((f) => (
                <Chip key={f} value={f} size="sm">
                  {f.toUpperCase()}
                </Chip>
              ))}
            </Group>
          </Chip.Group>
          <Group mt="lg" grow>
            <Button
              variant="default"
              onClick={() => {
                setAuthorFilter("");
                setTitleFilter("");
                setFormatFilter("");
              }}>
              Clear All
            </Button>
            <Button onClick={() => setFilterOpen(false)}>Done</Button>
          </Group>
        </Drawer>
      </Box>
    );
  }

  // ── Desktop: table view ──────────────────────────────────────────────────
  const virtualItems = rowVirtualizer.getVirtualItems();
  const paddingTop = virtualItems.length > 0 ? virtualItems[0]?.start || 0 : 0;
  const paddingBottom =
    virtualItems.length > 0
      ? rowVirtualizer.getTotalSize() -
        (virtualItems[virtualItems.length - 1]?.end || 0)
      : 0;

  return (
    <ScrollArea
      viewportRef={mergedRef}
      className={classes.container}
      type="hover"
      scrollbarSize={6}
      styles={{ thumb: { ["&::before"]: { minWidth: 4 } } }}
      offsetScrollbars={false}>
      <Table highlightOnHover verticalSpacing="sm" fontSize="xs">
        <thead className={classes.head}>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className={classes.headerCell}
                  style={{ width: header.getSize() }}>
                  {flexRender(
                    header.column.columnDef.header,
                    header.getContext()
                  )}
                  <div
                    onMouseDown={header.getResizeHandler()}
                    onTouchStart={header.getResizeHandler()}
                    className={cx(classes.resizer, {
                      ["isResizing"]: header.column.getIsResizing()
                    })}
                  />
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {paddingTop > 0 && (
            <tr>
              <td style={{ height: `${paddingTop}px` }} />
            </tr>
          )}
          {rowVirtualizer.getVirtualItems().map((virtualRow) => {
            const row = tableRows[virtualRow.index] as unknown as Row<BookDetail>;
            return (
              <tr key={row.id} style={{ height: 50 }}>
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id}>
                    <Text lineClamp={1} color="dark">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </Text>
                  </td>
                ))}
              </tr>
            );
          })}
          {paddingBottom > 0 && (
            <tr>
              <td style={{ height: `${paddingBottom}px` }} />
            </tr>
          )}
        </tbody>
      </Table>
    </ScrollArea>
  );
}

function CardDownloadButton({ book, author, title }: { book: string; author?: string; title?: string }) {
  const dispatch = useAppDispatch();
  const [clicked, setClicked] = useState(false);
  const isInFlight = useSelector((state: RootState) =>
    state.state.inFlightDownloads.includes(book)
  );

  const onClick = () => {
    if (clicked) return;
    dispatch(sendDownload({ book, author, title }));
    setClicked(true);
  };

  return (
    <ActionIcon
      color="brand"
      variant="filled"
      size="lg"
      radius="sm"
      onClick={onClick}
      disabled={clicked && !isInFlight}
      style={{ flexShrink: 0 }}>
      {isInFlight ? (
        <Loader size="xs" color="white" />
      ) : (
        <DownloadSimple size={20} weight="bold" />
      )}
    </ActionIcon>
  );
}

function DownloadButton({ book, author, title }: { book: string; author?: string; title?: string }) {
  const dispatch = useAppDispatch();
  const [clicked, setClicked] = useState(false);
  const isInFlight = useSelector((state: RootState) =>
    state.state.inFlightDownloads.includes(book)
  );

  const onClick = () => {
    if (clicked) return;
    dispatch(sendDownload({ book, author, title }));
    setClicked(true);
  };

  return (
    <Button
      compact
      size="xs"
      radius="sm"
      onClick={onClick}
      sx={{ fontWeight: "normal", width: 80 }}>
      {isInFlight ? (
        <Loader variant="dots" color="gray" />
      ) : (
        <span>Download</span>
      )}
    </Button>
  );
}
