// @ts-check

// Minimal structural fixture — validates the renderer can find and insert
// into all 4 section anchors. Does NOT need to track real page additions
// in pt-ekklesia-docs; only the anchor pattern matters.

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'category',
      label: 'Platform Grouping',
      link: { type: 'doc', id: 'platform-grouping/index' },
      items: [
        // region: platform-grouping
        {
          type: 'category',
          label: 'Logos',
          link: { type: 'doc', id: 'platform-grouping/logos/index' },
          items: [],
        },
        // endregion: platform-grouping
      ],
    },
    {
      type: 'category',
      label: 'Stream-Aligned Teams',
      link: { type: 'doc', id: 'stream-aligned-teams/index' },
      items: [
        // region: stream-aligned-teams
        // endregion: stream-aligned-teams
      ],
    },
    {
      type: 'category',
      label: 'Complicated Subsystem Teams',
      link: { type: 'doc', id: 'complicated-subsystem-teams/index' },
      items: [
        // region: complicated-subsystem-teams
        // endregion: complicated-subsystem-teams
      ],
    },
    {
      type: 'category',
      label: 'Enabling Teams',
      link: { type: 'doc', id: 'enabling-teams/index' },
      items: [
        // region: enabling-teams
        // endregion: enabling-teams
      ],
    },
  ],
};

export default sidebars;
